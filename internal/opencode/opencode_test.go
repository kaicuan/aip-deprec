package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aip/internal/db"
)

func TestBuildModelsDict(t *testing.T) {
	models := []*db.Model{
		{
			ID:            "test-model-1",
			Name:          "Test Model 1",
			ContextLength: 1048576,
			OutputLimit:   131072,
			Modalities:    "text,image,audio",
			Variants:      `{"low":{"reasoningEffort":"low"},"high":{"reasoningEffort":"high"}}`,
			Capabilities:  `{"reasoning":true,"temperature":true,"tool_call":true,"attachment":true}`,
		},
		{
			ID:            "test-model-2",
			Name:          "Test Model 2",
			ContextLength: 200000,
			OutputLimit:   32000,
			Modalities:    "",
		},
		{
			ID:            "muse-spark-1.3-contributor-free",
			Name:          "Muse Spark 1.3 Free",
			ContextLength: 1048576,
			OutputLimit:   131072,
			ProviderNPM:   "@ai-sdk/openai",
		},
	}

	dict := buildModelsDict(models)

	if len(dict) != 3 {
		t.Fatalf("expected 3 models, got %d", len(dict))
	}

	m1, ok := dict["test-model-1"].(map[string]interface{})
	if !ok {
		t.Fatalf("test-model-1 missing or wrong type")
	}
	if m1["name"] != "Test Model 1" {
		t.Errorf("expected name 'Test Model 1', got %v", m1["name"])
	}

	limit1, ok := m1["limit"].(map[string]interface{})
	if !ok {
		t.Fatalf("limit missing or wrong type")
	}
	if limit1["context"] != 1048576 {
		t.Errorf("expected context 1048576, got %v", limit1["context"])
	}
	if limit1["output"] != 131072 {
		t.Errorf("expected output 131072, got %v", limit1["output"])
	}

	modalities1, ok := m1["modalities"].(map[string]interface{})
	if !ok {
		t.Fatalf("modalities missing or wrong type")
	}
	inputs1, ok := modalities1["input"].([]string)
	if !ok || len(inputs1) != 3 {
		t.Fatalf("expected 3 input modalities, got %v", modalities1["input"])
	}

	// Test variants
	variants1, ok := m1["variants"].(map[string]interface{})
	if !ok {
		t.Fatalf("variants missing or wrong type")
	}
	lowVar, ok := variants1["low"].(map[string]interface{})
	if !ok || lowVar["reasoningEffort"] != "low" {
		t.Errorf("expected low reasoningEffort, got %v", lowVar)
	}

	// Test capabilities
	if m1["reasoning"] != true {
		t.Errorf("expected reasoning true, got %v", m1["reasoning"])
	}
	if m1["attachment"] != true {
		t.Errorf("expected attachment true, got %v", m1["attachment"])
	}

	// Test model 2 default modality and no provider override
	m2 := dict["test-model-2"].(map[string]interface{})
	modalities2 := m2["modalities"].(map[string]interface{})
	inputs2 := modalities2["input"].([]string)
	if len(inputs2) != 1 || inputs2[0] != "text" {
		t.Errorf("expected default ['text'] input modality, got %v", inputs2)
	}
	if _, hasProv := m2["provider"]; hasProv {
		t.Errorf("expected no provider override for test-model-2, got %v", m2["provider"])
	}

	// Test muse model provider override
	mMuse := dict["muse-spark-1.3-contributor-free"].(map[string]interface{})
	provMuse, ok := mMuse["provider"].(map[string]interface{})
	if !ok || provMuse["npm"] != "@ai-sdk/openai" {
		t.Errorf("expected provider.npm '@ai-sdk/openai' for muse model, got %v", mMuse["provider"])
	}
}

func TestLinkAndUnlink(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "opencode.json")

	mgr := NewManager(configPath)

	status, err := mgr.CheckStatus()
	if err != nil {
		t.Fatalf("CheckStatus error: %v", err)
	}
	if status.ConfigFileExists || status.IsLinked {
		t.Errorf("expected not exists and not linked")
	}

	models := []*db.Model{
		{
			ID:            "mimo-v2.6-flash-free",
			Name:          "MiMo-V2.6-Flash Free",
			ContextLength: 200000,
			OutputLimit:   32000,
			Modalities:    "text,image",
		},
	}

	if err := mgr.LinkOrSync(models); err != nil {
		t.Fatalf("LinkOrSync error: %v", err)
	}

	status, err = mgr.CheckStatus()
	if err != nil {
		t.Fatalf("CheckStatus error: %v", err)
	}
	if !status.ConfigFileExists || !status.IsLinked {
		t.Errorf("expected exists and linked")
	}
	if status.SyncedModelCount != 1 {
		t.Errorf("expected 1 synced model, got %d", status.SyncedModelCount)
	}

	// Verify file content structure
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	provider := root["provider"].(map[string]interface{})
	aip := provider["aip"].(map[string]interface{})
	modelsMap := aip["models"].(map[string]interface{})
	mimo := modelsMap["mimo-v2.6-flash-free"].(map[string]interface{})
	limit := mimo["limit"].(map[string]interface{})
	if limit["context"].(float64) != 200000 {
		t.Errorf("expected context 200000, got %v", limit["context"])
	}

	// Test Unlink
	if err := mgr.Unlink(); err != nil {
		t.Fatalf("Unlink error: %v", err)
	}

	status, err = mgr.CheckStatus()
	if err != nil {
		t.Fatalf("CheckStatus error: %v", err)
	}
	if status.IsLinked {
		t.Errorf("expected unlinked")
	}
}
