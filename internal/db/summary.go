package db

import (
	"fmt"
	"time"
)

type TimeBucketPoint struct {
	Bucket           string `json:"bucket"`
	Timestamp        int64  `json:"timestamp"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	Success          int64  `json:"success"`
	RateLimited      int64  `json:"rate_limited"`
	Errors           int64  `json:"errors"`
	TotalRequests    int64  `json:"total_requests"`
	AvgLatencyMs     int64  `json:"avg_latency_ms"`
}

type UsageSummary struct {
	Range                 string            `json:"range"`
	RangeMinutes          int64             `json:"range_minutes"`
	TotalRequests         int64             `json:"total_requests"`
	TotalTokens           int64             `json:"total_tokens"`
	TotalPromptTokens     int64             `json:"total_prompt_tokens"`
	TotalCompletionTokens int64             `json:"total_completion_tokens"`
	TotalCacheReadTokens  int64             `json:"total_cache_read_tokens"`
	CacheHitRate          float64           `json:"cache_hit_rate"`
	AvgTokensPerRequest   float64           `json:"avg_tokens_per_request"`
	RPM                   float64           `json:"rpm"`
	TPM                   float64           `json:"tpm"`
	EstimatedSavings      float64           `json:"estimated_savings"`
	AvgLatencyMs          int64             `json:"avg_latency_ms"`
	ModelUsage            map[string]int64  `json:"model_usage"`
	ModelTokens           map[string]int64  `json:"model_tokens"`
	ProxyUsage            map[string]int64  `json:"proxy_usage"`
	ProxyTokens           map[string]int64  `json:"proxy_tokens"`
	TimeBuckets           []TimeBucketPoint `json:"time_buckets"`
}

func (d *DB) GetUsageSummary(rangeStr string) (*UsageSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	now := time.Now().UTC()
	var cutoff int64
	var rangeMinutes int64
	var bucketSeconds int64
	var buckets []TimeBucketPoint
	bucketIndexMap := make(map[int64]int)

	switch rangeStr {
	case "today", "24h":
		rangeStr = "today"
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		cutoff = start.Unix()
		rangeMinutes = int64(now.Sub(start).Minutes())
		if rangeMinutes <= 0 {
			rangeMinutes = 1
		}
		bucketSeconds = 7200 // 2-hour intervals -> 12 points
		for h := 0; h < 24; h += 2 {
			t := start.Add(time.Duration(h) * time.Hour)
			label := fmt.Sprintf("%02d:00", h)
			bucketIndexMap[t.Unix()] = len(buckets)
			buckets = append(buckets, TimeBucketPoint{
				Bucket:    label,
				Timestamp: t.Unix(),
			})
		}

	case "monthly", "30d":
		rangeStr = "monthly"
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		cutoff = start.Unix()
		rangeMinutes = int64(now.Sub(start).Minutes())
		if rangeMinutes <= 0 {
			rangeMinutes = 1
		}
		bucketSeconds = 86400 // 1-day intervals -> ~30 points
		nextMonth := start.AddDate(0, 1, 0)
		daysInMonth := int(nextMonth.Sub(start).Hours() / 24)
		for d := 0; d < daysInMonth; d++ {
			t := start.AddDate(0, 0, d)
			label := t.Format("Jan 02")
			bucketIndexMap[t.Unix()] = len(buckets)
			buckets = append(buckets, TimeBucketPoint{
				Bucket:    label,
				Timestamp: t.Unix(),
			})
		}

	default: // "weekly", "7d"
		rangeStr = "weekly"
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		mondayOffset := weekday - 1
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -mondayOffset)
		cutoff = start.Unix()
		rangeMinutes = int64(now.Sub(start).Minutes())
		if rangeMinutes <= 0 {
			rangeMinutes = 1
		}
		bucketSeconds = 43200 // 12-hour intervals -> 14 points (Mon-Sun, 2 per day)
		for d := 0; d < 7; d++ {
			dayStart := start.AddDate(0, 0, d)
			for h := 0; h < 24; h += 12 {
				t := dayStart.Add(time.Duration(h) * time.Hour)
				label := fmt.Sprintf("%s %02d:00", t.Format("Mon"), h)
				bucketIndexMap[t.Unix()] = len(buckets)
				buckets = append(buckets, TimeBucketPoint{
					Bucket:    label,
					Timestamp: t.Unix(),
				})
			}
		}
	}

	summary := &UsageSummary{
		Range:        rangeStr,
		RangeMinutes: rangeMinutes,
		ModelUsage:   make(map[string]int64),
		ModelTokens:  make(map[string]int64),
		ProxyUsage:   make(map[string]int64),
		ProxyTokens:  make(map[string]int64),
		TimeBuckets:  buckets,
	}

	row := d.db.QueryRow(`SELECT COUNT(*),
		COALESCE(SUM(prompt_tokens), 0),
		COALESCE(SUM(completion_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0),
		COALESCE(SUM(estimated_savings), 0),
		COALESCE(AVG(duration_ms), 0)
		FROM request_logs WHERE timestamp >= ?`, cutoff)
	var totalReqs, promptTokens, compTokens, cacheTokens int64
	var estimatedSavings, avgLatency float64
	if err := row.Scan(&totalReqs, &promptTokens, &compTokens, &cacheTokens, &estimatedSavings, &avgLatency); err == nil {
		summary.TotalRequests = totalReqs
		if totalReqs > 0 {
			summary.AvgTokensPerRequest = float64(promptTokens+compTokens) / float64(totalReqs)
		}
		summary.TotalPromptTokens = promptTokens
		summary.TotalCompletionTokens = compTokens
		summary.TotalCacheReadTokens = cacheTokens
		summary.TotalTokens = promptTokens + compTokens
		summary.EstimatedSavings = estimatedSavings
		summary.AvgLatencyMs = int64(avgLatency)

		if promptTokens > 0 {
			summary.CacheHitRate = float64(cacheTokens) / float64(promptTokens) * 100.0
		}

		if rangeMinutes > 0 {
			summary.RPM = float64(totalReqs) / float64(rangeMinutes)
			summary.TPM = float64(summary.TotalTokens) / float64(rangeMinutes)
		}
	}

	// Time-series bucketing query using integer division on unix timestamps
	timeQuery := `SELECT 
		(timestamp / ?) * ? AS bkey,
		COALESCE(SUM(prompt_tokens), 0),
		COALESCE(SUM(completion_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0),
		COUNT(*),
		COALESCE(SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status_code = 429 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status_code != 429 AND (status_code < 200 OR status_code >= 300) THEN 1 ELSE 0 END), 0),
		COALESCE(AVG(duration_ms), 0)
		FROM request_logs WHERE timestamp >= ?
		GROUP BY bkey`

	if tRows, err := d.db.Query(timeQuery, bucketSeconds, bucketSeconds, cutoff); err == nil {
		defer tRows.Close()
		for tRows.Next() {
			var bkey int64
			var pTokens, cTokens, cacheTokens, reqs, succ, rateLim, errs int64
			var avgLat float64
			if err := tRows.Scan(&bkey, &pTokens, &cTokens, &cacheTokens, &reqs, &succ, &rateLim, &errs, &avgLat); err == nil {
				if idx, exists := bucketIndexMap[bkey]; exists {
					summary.TimeBuckets[idx].PromptTokens = pTokens
					summary.TimeBuckets[idx].CompletionTokens = cTokens
					summary.TimeBuckets[idx].TotalTokens = pTokens + cTokens
					summary.TimeBuckets[idx].CacheReadTokens = cacheTokens
					summary.TimeBuckets[idx].TotalRequests = reqs
					summary.TimeBuckets[idx].Success = succ
					summary.TimeBuckets[idx].RateLimited = rateLim
					summary.TimeBuckets[idx].Errors = errs
					summary.TimeBuckets[idx].AvgLatencyMs = int64(avgLat)
				}
			}
		}
	}

	// Model distribution query
	mRows, err := d.db.Query(`SELECT model, COUNT(*), COALESCE(SUM(prompt_tokens + completion_tokens), 0)
		FROM request_logs WHERE timestamp >= ? GROUP BY model`, cutoff)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var m string
			var c, tok int64
			if err := mRows.Scan(&m, &c, &tok); err == nil {
				summary.ModelUsage[m] = c
				summary.ModelTokens[m] = tok
			}
		}
	}

	// Proxy distribution query
	pRows, err := d.db.Query(`SELECT proxy_url, COUNT(*), COALESCE(SUM(prompt_tokens + completion_tokens), 0)
		FROM request_logs WHERE timestamp >= ? GROUP BY proxy_url`, cutoff)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var p string
			var c, tok int64
			if err := pRows.Scan(&p, &c, &tok); err == nil {
				summary.ProxyUsage[p] = c
				summary.ProxyTokens[p] = tok
			}
		}
	}

	return summary, nil
}
