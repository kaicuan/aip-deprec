package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"aip/internal/db"
)

type StatusResponse struct {
	ConfigFileExists bool     `json:"config_file_exists"`
	ConfigPath       string   `json:"config_path"`
	IsLinked         bool     `json:"is_linked"`
	SyncedModels     []string `json:"synced_models"`
}

type Manager struct {
	configPath string
	mu         sync.Mutex
}

func NewManager(configPath string) *Manager {
	return &Manager{configPath: configPath}
}

func (m *Manager) CheckStatus() (*StatusResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	res := &StatusResponse{
		ConfigPath:   m.configPath,
		SyncedModels: []string{},
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			res.ConfigFileExists = false
			res.IsLinked = false
			return res, nil
		}
		return nil, err
	}

	res.ConfigFileExists = true

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return res, nil
	}

	providerRaw, ok := root["provider"]
	if !ok {
		return res, nil
	}
	providerMap, ok := providerRaw.(map[string]interface{})
	if !ok {
		return res, nil
	}

	aipRaw, ok := providerMap["aip"]
	if !ok {
		return res, nil
	}

	res.IsLinked = true
	if aipMap, ok := aipRaw.(map[string]interface{}); ok {
		if modelsRaw, ok := aipMap["models"]; ok {
			if modelsMap, ok := modelsRaw.(map[string]interface{}); ok {
				for modelID := range modelsMap {
					res.SyncedModels = append(res.SyncedModels, modelID)
				}
				sort.Strings(res.SyncedModels)
			}
		}
	}

	return res, nil
}

func (m *Manager) LinkOrSync(models []*db.Model) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create opencode config dir: %w", err)
	}

	root := make(map[string]interface{})
	if data, err := os.ReadFile(m.configPath); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &root)
	}

	if _, ok := root["$schema"]; !ok {
		root["$schema"] = "https://opencode.ai/config.json"
	}

	var providerMap map[string]interface{}
	if pRaw, ok := root["provider"]; ok {
		if pMap, ok := pRaw.(map[string]interface{}); ok {
			providerMap = pMap
		}
	}
	if providerMap == nil {
		providerMap = make(map[string]interface{})
		root["provider"] = providerMap
	}

	modelsDict := buildModelsDict(models)

	aipEntry := map[string]interface{}{
		"npm": "@ai-sdk/openai-compatible",
		"options": map[string]interface{}{
			"baseURL": "http://localhost:9100/v1",
			"apiKey":  "public",
		},
		"models": modelsDict,
	}

	providerMap["aip"] = aipEntry

	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	output = append(output, '\n')
	return os.WriteFile(m.configPath, output, 0644)
}

func (m *Manager) Unlink() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}

	if pRaw, ok := root["provider"]; ok {
		if pMap, ok := pRaw.(map[string]interface{}); ok {
			delete(pMap, "aip")
		}
	}

	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}

	output = append(output, '\n')
	return os.WriteFile(m.configPath, output, 0644)
}

func GenerateSnippet(models []*db.Model) string {
	modelsDict := buildModelsDict(models)

	configSnippet := map[string]interface{}{
		"provider": map[string]interface{}{
			"aip": map[string]interface{}{
				"npm": "@ai-sdk/openai-compatible",
				"options": map[string]interface{}{
					"baseURL": "http://localhost:9100/v1",
					"apiKey":  "public",
				},
				"models": modelsDict,
			},
		},
	}

	bytes, _ := json.MarshalIndent(configSnippet, "", "  ")
	return string(bytes)
}

func buildVariants(reasoningOptionsJSON string, reasoning bool, modelID string) map[string]interface{} {
	if !reasoning {
		return nil
	}

	var effortValues []string
	if reasoningOptionsJSON != "" && reasoningOptionsJSON != "[]" {
		var options []struct {
			Type   string   `json:"type"`
			Values []string `json:"values,omitempty"`
		}
		if err := json.Unmarshal([]byte(reasoningOptionsJSON), &options); err == nil {
			for _, opt := range options {
				if opt.Type == "effort" && len(opt.Values) > 0 {
					effortValues = opt.Values
					break
				}
			}
		}
	}

	// If no explicit effort values, apply OpenCode's reasoning fallback
	if len(effortValues) == 0 {
		idLower := strings.ToLower(modelID)
		// Models with fixed/non-configurable reasoning do not expose effort variants in OpenCode
		nonEffortModels := []string{"deepseek-chat", "deepseek-r1", "minimax", "kimi", "qwen", "big-pickle", "nemotron", "mimo"}
		for _, ne := range nonEffortModels {
			if strings.Contains(idLower, ne) {
				return nil
			}
		}
		effortValues = []string{"low", "medium", "high"}
	}

	variants := make(map[string]interface{}, len(effortValues))
	for _, v := range effortValues {
		variants[v] = map[string]string{
			"reasoningEffort": v,
		}
	}
	return variants
}

func buildModelsDict(models []*db.Model) map[string]interface{} {
	modelsDict := make(map[string]interface{})
	for _, mdl := range models {
		mEntry := map[string]interface{}{
			"name": mdl.Name,
		}

		if mdl.Limit != "" && mdl.Limit != "{}" {
			var limitMap map[string]interface{}
			if err := json.Unmarshal([]byte(mdl.Limit), &limitMap); err == nil && len(limitMap) > 0 {
				mEntry["limit"] = limitMap
			}
		}

		var inputModalities []string
		if mdl.Modalities != "" {
			for _, part := range strings.Split(mdl.Modalities, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					inputModalities = append(inputModalities, part)
				}
			}
		}
		if len(inputModalities) == 0 {
			inputModalities = []string{"text"}
		}

		mEntry["modalities"] = map[string]interface{}{
			"input":  inputModalities,
			"output": []string{"text"},
		}

		// Capabilities as direct boolean flags
		if mdl.Reasoning {
			mEntry["reasoning"] = true
		}
		if mdl.ToolCall {
			mEntry["tool_call"] = true
		}
		if mdl.Temperature {
			mEntry["temperature"] = true
		}
		if mdl.Attachment {
			mEntry["attachment"] = true
		}

		// Dynamic variant generation for OpenCode
		if variants := buildVariants(mdl.ReasoningOptions, mdl.Reasoning, mdl.ID); len(variants) > 0 {
			mEntry["variants"] = variants
		}

		// Provider SDK adapter
		if mdl.Provider != "" && mdl.Provider != "{}" {
			var provMap map[string]interface{}
			if err := json.Unmarshal([]byte(mdl.Provider), &provMap); err == nil && len(provMap) > 0 {
				mEntry["provider"] = provMap
			}
		} else if strings.Contains(strings.ToLower(mdl.ID), "muse") {
			mEntry["provider"] = map[string]interface{}{
				"npm": "@ai-sdk/openai",
			}
		}

		modelsDict[mdl.ID] = mEntry
	}
	return modelsDict
}

