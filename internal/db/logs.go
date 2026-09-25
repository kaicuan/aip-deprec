package db

import (
	"time"
)

type RequestLog struct {
	ID               int64  `json:"id"`
	Timestamp        int64  `json:"timestamp"`
	Model            string `json:"model"`
	ProxyURL         string `json:"proxy_url"`
	StatusCode       int    `json:"status_code"`
	DurationMs       int    `json:"duration_ms"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	CacheReadTokens  int    `json:"cache_read_tokens"`
	ErrorMessage     string `json:"error_message"`
	IsStream         int    `json:"is_stream"`
	SessionID        string `json:"session_id"`
	RequestBody      string `json:"request_body"`
	ResponseBody     string `json:"response_body"`
}

type Stats struct {
	TotalRequests      int64            `json:"total_requests"`
	SuccessRate        float64          `json:"success_rate"`
	TotalTokens        int64            `json:"total_tokens"`
	TotalPromptTokens  int64            `json:"total_prompt_tokens"`
	TotalCompTokens    int64            `json:"total_completion_tokens"`
	ActiveProxiesCount int64            `json:"active_proxies_count"`
	TotalProxiesCount  int64            `json:"total_proxies_count"`
	ModelUsage         map[string]int64 `json:"model_usage"`
}

func (d *DB) InsertRequestLog(log *RequestLog) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if log.Timestamp == 0 {
		log.Timestamp = time.Now().Unix()
	}
	res, err := d.db.Exec(`INSERT INTO request_logs (timestamp, model, proxy_url, status_code, duration_ms, prompt_tokens, completion_tokens, cache_read_tokens, error_message, is_stream, session_id, request_body, response_body)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.Timestamp, log.Model, log.ProxyURL, log.StatusCode, log.DurationMs, log.PromptTokens, log.CompletionTokens, log.CacheReadTokens, log.ErrorMessage, log.IsStream, log.SessionID, log.RequestBody, log.ResponseBody)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		log.ID = id
	}
	return nil
}

func (d *DB) ListRequestLogs(limit, offset int) ([]*RequestLog, int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var total int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM request_logs`).Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	rows, err := d.db.Query(`SELECT id, timestamp, model, proxy_url, status_code, duration_ms, prompt_tokens, completion_tokens, cache_read_tokens, error_message, is_stream, session_id
		FROM request_logs ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*RequestLog
	for rows.Next() {
		var l RequestLog
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.Model, &l.ProxyURL, &l.StatusCode, &l.DurationMs, &l.PromptTokens, &l.CompletionTokens, &l.CacheReadTokens, &l.ErrorMessage, &l.IsStream, &l.SessionID); err != nil {
			return nil, 0, err
		}
		logs = append(logs, &l)
	}
	return logs, total, nil
}

func (d *DB) GetRequestLog(id int64) (*RequestLog, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var l RequestLog
	err := d.db.QueryRow(`SELECT id, timestamp, model, proxy_url, status_code, duration_ms, prompt_tokens, completion_tokens, cache_read_tokens, error_message, is_stream, session_id, request_body, response_body
		FROM request_logs WHERE id = ?`, id).
		Scan(&l.ID, &l.Timestamp, &l.Model, &l.ProxyURL, &l.StatusCode, &l.DurationMs, &l.PromptTokens, &l.CompletionTokens, &l.CacheReadTokens, &l.ErrorMessage, &l.IsStream, &l.SessionID, &l.RequestBody, &l.ResponseBody)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (d *DB) GetStats() (*Stats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := &Stats{
		ModelUsage: make(map[string]int64),
	}

	row := d.db.QueryRow(`SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN status_code >= 200 AND status_code < 400 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(prompt_tokens), 0),
		COALESCE(SUM(completion_tokens), 0)
		FROM request_logs`)
	var totalReqs, successReqs, promptTokens, compTokens int64
	if err := row.Scan(&totalReqs, &successReqs, &promptTokens, &compTokens); err == nil {
		stats.TotalRequests = totalReqs
		if totalReqs > 0 {
			stats.SuccessRate = float64(successReqs) / float64(totalReqs) * 100.0
		} else {
			stats.SuccessRate = 100.0
		}
		stats.TotalPromptTokens = promptTokens
		stats.TotalCompTokens = compTokens
		stats.TotalTokens = promptTokens + compTokens
	}

	_ = d.db.QueryRow(`SELECT COUNT(*) FROM proxies`).Scan(&stats.TotalProxiesCount)
	now := time.Now().Unix()
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM proxies WHERE status = 'online' OR (status = 'rate_limited' AND cooldown_until > 0 AND cooldown_until <= ?)`, now).Scan(&stats.ActiveProxiesCount)

	rows, err := d.db.Query(`SELECT model, COUNT(*) FROM request_logs GROUP BY model`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m string
			var c int64
			if err := rows.Scan(&m, &c); err == nil {
				stats.ModelUsage[m] = c
			}
		}
	}

	return stats, nil
}
