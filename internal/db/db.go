package db

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Proxy struct {
	ID            int64  `json:"id"`
	RawInput      string `json:"raw_input"`
	URL           string `json:"url"`
	Protocol      string `json:"protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	Status        string `json:"status"` // "online", "rate_limited", "error"
	LatencyMs     int    `json:"latency_ms"`
	EgressIP      string `json:"egress_ip"`
	CooldownUntil int64  `json:"cooldown_until"` // Unix timestamp in seconds
	LastCheckedAt int64  `json:"last_checked_at"`
	FailCount     int    `json:"fail_count"`
	SuccessCount  int    `json:"success_count"`
	CreatedAt     int64  `json:"created_at"`
}

type Model struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	Lab           string `json:"lab"`
	ContextLength int    `json:"context_length"`
	OutputLimit   int    `json:"output_limit"`
	Modalities    string `json:"modalities"` // e.g. "text,image"
	IsFree        int    `json:"is_free"`
	FetchedAt     int64  `json:"fetched_at"`
	Variants      string `json:"variants"`     // JSON string of variants, e.g. {"low":{"reasoningEffort":"low"}}
	Capabilities  string `json:"capabilities"` // JSON string of reasoning, temperature, tool_call, attachment
	ProviderNPM   string `json:"provider_npm"` // SDK provider override, e.g. "@ai-sdk/openai"
}

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
	TotalRequests      int64   `json:"total_requests"`
	SuccessRate        float64 `json:"success_rate"`
	TotalTokens        int64   `json:"total_tokens"`
	TotalPromptTokens  int64   `json:"total_prompt_tokens"`
	TotalCompTokens    int64   `json:"total_completion_tokens"`
	ActiveProxiesCount int64   `json:"active_proxies_count"`
	TotalProxiesCount  int64   `json:"total_proxies_count"`
	ModelUsage         map[string]int64 `json:"model_usage"`
}

type DB struct {
	db *sql.DB
	mu sync.RWMutex
}

func Open(dbPath string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	d := &DB{db: sqlDB}
	if err := d.initSchema(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return d, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS proxies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			raw_input TEXT NOT NULL,
			url TEXT UNIQUE NOT NULL,
			protocol TEXT NOT NULL,
			host TEXT NOT NULL,
			port INTEGER NOT NULL,
			username TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'online',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			egress_ip TEXT NOT NULL DEFAULT '',
			cooldown_until INTEGER NOT NULL DEFAULT 0,
			last_checked_at INTEGER NOT NULL DEFAULT 0,
			fail_count INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS models (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			lab TEXT NOT NULL DEFAULT '',
			context_length INTEGER NOT NULL DEFAULT 0,
			output_limit INTEGER NOT NULL DEFAULT 0,
			modalities TEXT NOT NULL DEFAULT '',
			is_free INTEGER NOT NULL DEFAULT 1,
			fetched_at INTEGER NOT NULL DEFAULT 0,
			variants TEXT NOT NULL DEFAULT '',
			capabilities TEXT NOT NULL DEFAULT '',
			provider_npm TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp INTEGER NOT NULL,
			model TEXT NOT NULL,
			proxy_url TEXT NOT NULL,
			status_code INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT '',
			is_stream INTEGER NOT NULL DEFAULT 0,
			session_id TEXT NOT NULL DEFAULT '',
			request_body TEXT NOT NULL DEFAULT '',
			response_body TEXT NOT NULL DEFAULT ''
		);`,
	}

	for _, q := range queries {
		if _, err := d.db.Exec(q); err != nil {
			return err
		}
	}

	// Auto-migrate models table for existing databases
	_ = d.addColumnIfNotExists("models", "lab", "TEXT NOT NULL DEFAULT ''")
	_ = d.addColumnIfNotExists("models", "variants", "TEXT NOT NULL DEFAULT ''")
	_ = d.addColumnIfNotExists("models", "capabilities", "TEXT NOT NULL DEFAULT ''")
	_ = d.addColumnIfNotExists("models", "provider_npm", "TEXT NOT NULL DEFAULT ''")

	// Migrate optional columns if existing DB was created without them
	_ = d.addColumnIfNotExists("models", "output_limit", "INTEGER NOT NULL DEFAULT 0")
	_ = d.addColumnIfNotExists("models", "modalities", "TEXT NOT NULL DEFAULT ''")
	_ = d.addColumnIfNotExists("request_logs", "cache_read_tokens", "INTEGER NOT NULL DEFAULT 0")
	_ = d.addColumnIfNotExists("request_logs", "session_id", "TEXT NOT NULL DEFAULT ''")
	_ = d.addColumnIfNotExists("request_logs", "request_body", "TEXT NOT NULL DEFAULT ''")
	_ = d.addColumnIfNotExists("request_logs", "response_body", "TEXT NOT NULL DEFAULT ''")
	// Clean up any direct proxy entries so local IP is never part of the pool
	_, _ = d.db.Exec(`DELETE FROM proxies WHERE protocol = 'direct'`)

	return nil
}

func (d *DB) addColumnIfNotExists(table, column, colType string) error {
	query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType)
	_, err := d.db.Exec(query)
	return err
}

func (d *DB) ListProxies() ([]*Proxy, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT id, raw_input, url, protocol, host, port, username, password, status, latency_ms, egress_ip, cooldown_until, last_checked_at, fail_count, success_count, created_at FROM proxies ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proxies []*Proxy
	now := time.Now().Unix()
	for rows.Next() {
		var p Proxy
		if err := rows.Scan(&p.ID, &p.RawInput, &p.URL, &p.Protocol, &p.Host, &p.Port, &p.Username, &p.Password, &p.Status, &p.LatencyMs, &p.EgressIP, &p.CooldownUntil, &p.LastCheckedAt, &p.FailCount, &p.SuccessCount, &p.CreatedAt); err != nil {
			return nil, err
		}
		// Auto-recover rate limited proxy if cooldown expired
		if p.Status == "rate_limited" && p.CooldownUntil > 0 && now >= p.CooldownUntil {
			p.Status = "online"
			p.CooldownUntil = 0
			go d.UpdateProxyStatus(p.ID, "online", p.LatencyMs, p.EgressIP, 0, "")
		}
		proxies = append(proxies, &p)
	}
	return proxies, nil
}

func (d *DB) GetProxy(id int64) (*Proxy, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var p Proxy
	err := d.db.QueryRow(`SELECT id, raw_input, url, protocol, host, port, username, password, status, latency_ms, egress_ip, cooldown_until, last_checked_at, fail_count, success_count, created_at FROM proxies WHERE id = ?`, id).
		Scan(&p.ID, &p.RawInput, &p.URL, &p.Protocol, &p.Host, &p.Port, &p.Username, &p.Password, &p.Status, &p.LatencyMs, &p.EgressIP, &p.CooldownUntil, &p.LastCheckedAt, &p.FailCount, &p.SuccessCount, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *DB) InsertProxy(p *Proxy) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().Unix()
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	res, err := d.db.Exec(`INSERT INTO proxies (raw_input, url, protocol, host, port, username, password, status, latency_ms, egress_ip, cooldown_until, last_checked_at, fail_count, success_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.RawInput, p.URL, p.Protocol, p.Host, p.Port, p.Username, p.Password, p.Status, p.LatencyMs, p.EgressIP, p.CooldownUntil, p.LastCheckedAt, p.FailCount, p.SuccessCount, p.CreatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		p.ID = id
	}
	return nil
}

func (d *DB) UpdateProxy(p *Proxy) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE proxies SET raw_input=?, url=?, protocol=?, host=?, port=?, username=?, password=?, status=?, latency_ms=?, egress_ip=?, cooldown_until=?, last_checked_at=?, fail_count=?, success_count=? WHERE id=?`,
		p.RawInput, p.URL, p.Protocol, p.Host, p.Port, p.Username, p.Password, p.Status, p.LatencyMs, p.EgressIP, p.CooldownUntil, p.LastCheckedAt, p.FailCount, p.SuccessCount, p.ID)
	return err
}

func (d *DB) UpdateProxyStatus(id int64, status string, latencyMs int, egressIP string, cooldownUntil int64, _ string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().Unix()
	_, err := d.db.Exec(`UPDATE proxies SET status = ?, latency_ms = ?, egress_ip = ?, cooldown_until = ?, last_checked_at = ? WHERE id = ?`,
		status, latencyMs, egressIP, cooldownUntil, now, id)
	return err
}

func (d *DB) RecordProxySuccess(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().Unix()
	_, err := d.db.Exec(`UPDATE proxies SET status = 'online', success_count = success_count + 1, cooldown_until = 0, last_checked_at = ? WHERE id = ?`, now, id)
	return err
}

func (d *DB) RecordProxyFailure(id int64, cooldownUntil int64, is429 bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().Unix()
	status := "error"
	if is429 {
		status = "rate_limited"
	}
	_, err := d.db.Exec(`UPDATE proxies SET status = ?, fail_count = fail_count + 1, cooldown_until = ?, last_checked_at = ? WHERE id = ?`, status, cooldownUntil, now, id)
	return err
}

func (d *DB) DeleteProxy(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM proxies WHERE id = ?`, id)
	return err
}

func (d *DB) DeleteProxies(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf("DELETE FROM proxies WHERE id IN (%s)", strings.Join(placeholders, ","))
	_, err := d.db.Exec(query, args...)
	return err
}

func (d *DB) ListModels() ([]*Model, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT id, name, provider, lab, context_length, output_limit, modalities, is_free, fetched_at, variants, capabilities, provider_npm FROM models ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []*Model
	for rows.Next() {
		var m Model
		if err := rows.Scan(&m.ID, &m.Name, &m.Provider, &m.Lab, &m.ContextLength, &m.OutputLimit, &m.Modalities, &m.IsFree, &m.FetchedAt, &m.Variants, &m.Capabilities, &m.ProviderNPM); err != nil {
			return nil, err
		}
		models = append(models, &m)
	}
	return models, nil
}

// UpdateModels atomically replaces the active models catalog inside a single transaction
func (d *DB) UpdateModels(models []*Model) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM models`); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO models (id, name, provider, lab, context_length, output_limit, modalities, is_free, fetched_at, variants, capabilities, provider_npm)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, m := range models {
		fetchedAt := m.FetchedAt
		if fetchedAt == 0 {
			fetchedAt = now
		}
		if _, err := stmt.Exec(m.ID, m.Name, m.Provider, m.Lab, m.ContextLength, m.OutputLimit, m.Modalities, m.IsFree, fetchedAt, m.Variants, m.Capabilities, m.ProviderNPM); err != nil {
			return err
		}
	}

	return tx.Commit()
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
	rows, err := d.db.Query(`SELECT id, timestamp, model, proxy_url, status_code, duration_ms, prompt_tokens, completion_tokens, cache_read_tokens, error_message, is_stream, session_id, request_body, response_body
		FROM request_logs ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*RequestLog
	for rows.Next() {
		var l RequestLog
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.Model, &l.ProxyURL, &l.StatusCode, &l.DurationMs, &l.PromptTokens, &l.CompletionTokens, &l.CacheReadTokens, &l.ErrorMessage, &l.IsStream, &l.SessionID, &l.RequestBody, &l.ResponseBody); err != nil {
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
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM proxies WHERE status = 'online' OR (status = 'rate_limited' AND cooldown_until > 0 AND cooldown_until <= ?)` , now).Scan(&stats.ActiveProxiesCount)

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
