package db

import (
	"fmt"
	"strings"
	"time"
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

func (d *DB) ListProxies() ([]*Proxy, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT id, raw_input, url, protocol, host, port, username, password, status, latency_ms, egress_ip, cooldown_until, last_checked_at, fail_count, success_count, created_at FROM proxies ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proxies []*Proxy
	for rows.Next() {
		var p Proxy
		if err := rows.Scan(&p.ID, &p.RawInput, &p.URL, &p.Protocol, &p.Host, &p.Port, &p.Username, &p.Password, &p.Status, &p.LatencyMs, &p.EgressIP, &p.CooldownUntil, &p.LastCheckedAt, &p.FailCount, &p.SuccessCount, &p.CreatedAt); err != nil {
			return nil, err
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
