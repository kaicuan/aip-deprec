package db

import (
	"time"
)

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
