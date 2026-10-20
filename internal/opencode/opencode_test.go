package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aip/internal/db"
)

func TestBuildModelsDict(t *testing.T) {
	t.Run("ModelWithLimitsAndCapabilities", func(t *testing.T) {
		models := []*db.Model{
			{
				ID:               "custom-llm",
				Name:             "Custom LLM",
				Limit:            `{"context":128000,"output":4096}`,
				Modalities:       "text,image",
				Reasoning:        true,
				ReasoningOptions: `[{"type":"effort","values":["low"]}]`,
			},
		}

		dict := buildModelsDict(models)
		entry, ok := dict["custom-llm"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected custom-llm in dictionary")
		}

		if entry["name"] != "Custom LLM" {
			t.Errorf("expected name 'Custom LLM', got %v", entry["name"])
		}

		limit, ok := entry["limit"].(map[string]interface{})
		if !ok || int(limit["context"].(float64)) != 128000 || int(limit["output"].(float64)) != 4096 {
			t.Errorf("unexpected limit: %v", limit)
		}

		if entry["reasoning"] != true {
			t.Errorf("expected reasoning capability true, got %v", entry["reasoning"])
		}

		variants, ok := entry["variants"].(map[string]interface{})
		if !ok || variants["low"] == nil {
			t.Errorf("expected variants.low, got %v", entry["variants"])
		}
	})

	t.Run("DefaultTextModality", func(t *testing.T) {
		models := []*db.Model{
			{
				ID:   "simple-model",
				Name: "Simple Model",
			},
		}

		dict := buildModelsDict(models)
		entry := dict["simple-model"].(map[string]interface{})
		modalities := entry["modalities"].(map[string]interface{})
		inputs := modalities["input"].([]string)

		if len(inputs) != 1 || inputs[0] != "text" {
			t.Errorf("expected default input modality ['text'], got %v", inputs)
		}
	})

	t.Run("ProviderOverride", func(t *testing.T) {
		models := []*db.Model{
			{
				ID:       "special-model",
				Name:     "Special Model",
				Provider: `{"npm":"@ai-sdk/openai"}`,
			},
		}

		dict := buildModelsDict(models)
		entry := dict["special-model"].(map[string]interface{})
		prov, ok := entry["provider"].(map[string]interface{})
		if !ok || prov["npm"] != "@ai-sdk/openai" {
			t.Errorf("expected provider.npm '@ai-sdk/openai', got %v", entry["provider"])
		}
	})
}

func TestLinkAndUnlink(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "opencode.json")

	// Pre-populate with another existing provider to test non-destructive editing
	initialConfig := map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
		"provider": map[string]interface{}{
			"existing-provider": map[string]interface{}{
				"npm": "@ai-sdk/anthropic",
			},
		},
	}
	initialBytes, _ := json.MarshalIndent(initialConfig, "", "  ")
	_ = os.WriteFile(configPath, initialBytes, 0644)

	mgr := NewManager(configPath)

	t.Run("InitialUnlinkedState", func(t *testing.T) {
		status, err := mgr.CheckStatus()
		if err != nil {
			t.Fatalf("CheckStatus error: %v", err)
		}
		if !status.ConfigFileExists {
			t.Errorf("expected config file to exist")
		}
		if status.IsLinked {
			t.Errorf("expected is_linked = false initially")
		}
	})

	t.Run("LinkOrSync", func(t *testing.T) {
		models := []*db.Model{
			{
				ID:    "demo-free-model",
				Name:  "Demo Free Model",
				Limit: `{"context":100000}`,
			},
		}

		if err := mgr.LinkOrSync(models); err != nil {
			t.Fatalf("LinkOrSync error: %v", err)
		}

		status, err := mgr.CheckStatus()
		if err != nil {
			t.Fatalf("CheckStatus error: %v", err)
		}
		if !status.IsLinked {
			t.Errorf("expected is_linked = true after LinkOrSync")
		}
		if len(status.SyncedModels) != 1 {
			t.Errorf("expected 1 synced model, got %d", len(status.SyncedModels))
		}

		// Verify existing provider was NOT overwritten
		data, _ := os.ReadFile(configPath)
		var root map[string]interface{}
		_ = json.Unmarshal(data, &root)
		providers := root["provider"].(map[string]interface{})
		if _, exists := providers["existing-provider"]; !exists {
			t.Errorf("expected existing-provider to be preserved")
		}
		if _, exists := providers["aip"]; !exists {
			t.Errorf("expected aip provider to be created")
		}
	})

	t.Run("Unlink", func(t *testing.T) {
		if err := mgr.Unlink(); err != nil {
			t.Fatalf("Unlink error: %v", err)
		}

		status, err := mgr.CheckStatus()
		if err != nil {
			t.Fatalf("CheckStatus error: %v", err)
		}
		if status.IsLinked {
			t.Errorf("expected is_linked = false after Unlink")
		}

		// Verify existing provider is still there
		data, _ := os.ReadFile(configPath)
		var root map[string]interface{}
		_ = json.Unmarshal(data, &root)
		providers := root["provider"].(map[string]interface{})
		if _, exists := providers["existing-provider"]; !exists {
			t.Errorf("expected existing-provider to still be preserved")
		}
		if _, exists := providers["aip"]; exists {
			t.Errorf("expected aip provider to be removed")
		}
	})
}
