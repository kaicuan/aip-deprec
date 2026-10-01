package db

import (
	"time"
)

type ServerHealth struct {
	Status        string `json:"status,omitempty"`
	Uptime        string `json:"uptime,omitempty"`
	ActiveProxies int64  `json:"active_proxies"`
	TotalProxies  int64  `json:"total_proxies"`
}

func (d *DB) GetServerHealth() (*ServerHealth, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	health := &ServerHealth{}
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM proxies`).Scan(&health.TotalProxies)
	nowUnix := time.Now().Unix()
	_ = d.db.QueryRow(`SELECT COUNT(*) FROM proxies WHERE status = 'online' OR (status = 'rate_limited' AND cooldown_until > 0 AND cooldown_until <= ?)`, nowUnix).Scan(&health.ActiveProxies)

	return health, nil
}
