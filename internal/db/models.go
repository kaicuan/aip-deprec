package db

import (
	"time"
)

type Model struct {
	// 1. Raw / Direct from models.dev
	ID               string `json:"id"`
	Name             string `json:"name"`
	Limit            string `json:"limit"`             // Raw JSON: {"context": 262144, "output": 32768, "input": 200000}
	Modalities       string `json:"modalities"`        // Input modalities: "text", "text,image"
	Reasoning        bool   `json:"reasoning"`
	ToolCall         bool   `json:"tool_call"`
	Temperature      bool   `json:"temperature"`
	Attachment       bool   `json:"attachment"`
	ReasoningOptions string `json:"reasoning_options"` // Raw JSON: [{"type": "toggle"}], etc.
	Provider         string `json:"provider"`          // Raw JSON: {"npm": "@ai-sdk/openai"} or ""

	// 2. Derived / Proxy Metadata
	Lab            string `json:"lab"`             // Canonical creator lab: "ling", "google", "deepseek"
	CommercialRate string `json:"commercial_rate"` // Commercial equivalent rate JSON: {"input": X, "output": Y, "cache_read": Z}
	FetchedAt      int64  `json:"fetched_at"`      // Discovery timestamp (unix epoch)
}

func (d *DB) ListModels() ([]*Model, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT id, name, "limit", modalities, reasoning, tool_call, temperature, attachment, reasoning_options, provider, lab, commercial_rate, fetched_at FROM models ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []*Model
	for rows.Next() {
		var m Model
		if err := rows.Scan(&m.ID, &m.Name, &m.Limit, &m.Modalities, &m.Reasoning, &m.ToolCall, &m.Temperature, &m.Attachment, &m.ReasoningOptions, &m.Provider, &m.Lab, &m.CommercialRate, &m.FetchedAt); err != nil {
			return nil, err
		}
		models = append(models, &m)
	}
	return models, nil
}

func (d *DB) GetModel(id string) (*Model, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var m Model
	err := d.db.QueryRow(`SELECT id, name, "limit", modalities, reasoning, tool_call, temperature, attachment, reasoning_options, provider, lab, commercial_rate, fetched_at FROM models WHERE id = ?`, id).
		Scan(&m.ID, &m.Name, &m.Limit, &m.Modalities, &m.Reasoning, &m.ToolCall, &m.Temperature, &m.Attachment, &m.ReasoningOptions, &m.Provider, &m.Lab, &m.CommercialRate, &m.FetchedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
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

	stmt, err := tx.Prepare(`INSERT INTO models (id, name, "limit", modalities, reasoning, tool_call, temperature, attachment, reasoning_options, provider, lab, commercial_rate, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
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
		commercialRate := m.CommercialRate
		if commercialRate == "" {
			commercialRate = "{}"
		}
		limit := m.Limit
		if limit == "" {
			limit = "{}"
		}
		if _, err := stmt.Exec(m.ID, m.Name, limit, m.Modalities, m.Reasoning, m.ToolCall, m.Temperature, m.Attachment, m.ReasoningOptions, m.Provider, m.Lab, commercialRate, fetchedAt); err != nil {
			return err
		}
	}

	return tx.Commit()
}
