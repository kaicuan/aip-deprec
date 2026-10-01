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
	SessionID        string  `json:"session_id"`
	RequestBody      string  `json:"request_body"`
	ResponseBody     string  `json:"response_body"`
	EstimatedSavings float64 `json:"estimated_savings"`
}

func (d *DB) InsertRequestLog(log *RequestLog) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if log.Timestamp == 0 {
		log.Timestamp = time.Now().Unix()
	}
	res, err := d.db.Exec(`INSERT INTO request_logs (timestamp, model, proxy_url, status_code, duration_ms, prompt_tokens, completion_tokens, cache_read_tokens, error_message, is_stream, session_id, request_body, response_body, estimated_savings)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		log.Timestamp, log.Model, log.ProxyURL, log.StatusCode, log.DurationMs, log.PromptTokens, log.CompletionTokens, log.CacheReadTokens, log.ErrorMessage, log.IsStream, log.SessionID, log.RequestBody, log.ResponseBody, log.EstimatedSavings)
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
	rows, err := d.db.Query(`SELECT id, timestamp, model, proxy_url, status_code, duration_ms, prompt_tokens, completion_tokens, cache_read_tokens, error_message, is_stream, session_id, estimated_savings
		FROM request_logs ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*RequestLog
	for rows.Next() {
		var l RequestLog
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.Model, &l.ProxyURL, &l.StatusCode, &l.DurationMs, &l.PromptTokens, &l.CompletionTokens, &l.CacheReadTokens, &l.ErrorMessage, &l.IsStream, &l.SessionID, &l.EstimatedSavings); err != nil {
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
	err := d.db.QueryRow(`SELECT id, timestamp, model, proxy_url, status_code, duration_ms, prompt_tokens, completion_tokens, cache_read_tokens, error_message, is_stream, session_id, request_body, response_body, estimated_savings
		FROM request_logs WHERE id = ?`, id).
		Scan(&l.ID, &l.Timestamp, &l.Model, &l.ProxyURL, &l.StatusCode, &l.DurationMs, &l.PromptTokens, &l.CompletionTokens, &l.CacheReadTokens, &l.ErrorMessage, &l.IsStream, &l.SessionID, &l.RequestBody, &l.ResponseBody, &l.EstimatedSavings)
	if err != nil {
		return nil, err
	}
	return &l, nil
}
