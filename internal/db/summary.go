package db

import (
	"time"
)

type UsageSummary struct {
	Range                 string           `json:"range"`
	RangeMinutes          int64            `json:"range_minutes"`
	TotalRequests         int64            `json:"total_requests"`
	TotalTokens           int64            `json:"total_tokens"`
	TotalPromptTokens     int64            `json:"total_prompt_tokens"`
	TotalCompletionTokens int64            `json:"total_completion_tokens"`
	TotalCacheReadTokens  int64            `json:"total_cache_read_tokens"`
	CacheHitRate          float64          `json:"cache_hit_rate"`
	AvgTokensPerRequest   float64          `json:"avg_tokens_per_request"`
	RPM                   float64          `json:"rpm"`
	TPM                   float64          `json:"tpm"`
	EstimatedSavings      float64          `json:"estimated_savings"`
	ModelUsage            map[string]int64 `json:"model_usage"`
}

func (d *DB) GetUsageSummary(rangeStr string) (*UsageSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var rangeMinutes int64
	var cutoff int64
	now := time.Now()

	switch rangeStr {
	case "24h":
		rangeStr = "24h"
		rangeMinutes = 24 * 60
		cutoff = now.Add(-24 * time.Hour).Unix()
	case "30d":
		rangeStr = "30d"
		rangeMinutes = 30 * 24 * 60
		cutoff = now.Add(-30 * 24 * time.Hour).Unix()
	default:
		rangeStr = "7d"
		rangeMinutes = 7 * 24 * 60
		cutoff = now.Add(-7 * 24 * time.Hour).Unix()
	}

	summary := &UsageSummary{
		Range:        rangeStr,
		RangeMinutes: rangeMinutes,
		ModelUsage:   make(map[string]int64),
	}

	row := d.db.QueryRow(`SELECT COUNT(*),
		COALESCE(SUM(prompt_tokens), 0),
		COALESCE(SUM(completion_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0),
		COALESCE(SUM(estimated_savings), 0)
		FROM request_logs WHERE timestamp >= ?`, cutoff)
	var totalReqs, promptTokens, compTokens, cacheTokens int64
	var estimatedSavings float64
	if err := row.Scan(&totalReqs, &promptTokens, &compTokens, &cacheTokens, &estimatedSavings); err == nil {
		summary.TotalRequests = totalReqs
		if totalReqs > 0 {
			summary.AvgTokensPerRequest = float64(promptTokens+compTokens) / float64(totalReqs)
		}
		summary.TotalPromptTokens = promptTokens
		summary.TotalCompletionTokens = compTokens
		summary.TotalCacheReadTokens = cacheTokens
		summary.TotalTokens = promptTokens + compTokens
		summary.EstimatedSavings = estimatedSavings

		if promptTokens > 0 {
			summary.CacheHitRate = float64(cacheTokens) / float64(promptTokens) * 100.0
		}

		if rangeMinutes > 0 {
			summary.RPM = float64(totalReqs) / float64(rangeMinutes)
			summary.TPM = float64(summary.TotalTokens) / float64(rangeMinutes)
		}
	}

	rows, err := d.db.Query(`SELECT model, COUNT(*) FROM request_logs WHERE timestamp >= ? GROUP BY model`, cutoff)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m string
			var c int64
			if err := rows.Scan(&m, &c); err == nil {
				summary.ModelUsage[m] = c
			}
		}
	}

	return summary, nil
}
