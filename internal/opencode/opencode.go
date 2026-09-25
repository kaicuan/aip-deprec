package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"aip/internal/db"
)

type StatusResponse struct {
	ConfigFileExists bool     `json:"config_file_exists"`
	ConfigPath       string   `json:"config_path"`
	IsLinked         bool     `json:"is_linked"`
	SyncedModelCount int      `json:"synced_model_count"`
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
				res.SyncedModelCount = len(modelsMap)
				for modelID := range modelsMap {
					res.SyncedModels = append(res.SyncedModels, modelID)
				}
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

func (m *Manager) AutoSyncIfLinked(models []*db.Model) (bool, error) {
	status, err := m.CheckStatus()
	if err != nil || !status.IsLinked {
		return false, err
	}

	if err := m.LinkOrSync(models); err != nil {
		return false, err
	}
	return true, nil
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

func buildModelsDict(models []*db.Model) map[string]interface{} {
	modelsDict := make(map[string]interface{})
	for _, mdl := range models {
		mEntry := map[string]interface{}{
			"name": mdl.Name,
		}

		if mdl.ContextLength > 0 || mdl.OutputLimit > 0 {
			limitMap := make(map[string]interface{})
			if mdl.ContextLength > 0 {
				limitMap["context"] = mdl.ContextLength
			}
			if mdl.OutputLimit > 0 {
				limitMap["output"] = mdl.OutputLimit
			}
			mEntry["limit"] = limitMap
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

		// Add variants if present
		if mdl.Variants != "" {
			var variantsMap map[string]interface{}
			if err := json.Unmarshal([]byte(mdl.Variants), &variantsMap); err == nil {
				mEntry["variants"] = variantsMap
			}
		}

		// Add capabilities (reasoning, temperature, tool_call, attachment) if present
		if mdl.Capabilities != "" {
			var capsMap map[string]interface{}
			if err := json.Unmarshal([]byte(mdl.Capabilities), &capsMap); err == nil {
				for k, v := range capsMap {
					mEntry[k] = v
				}
			}
		}

		// Add provider SDK adapter override if specified (e.g. "@ai-sdk/openai" for Muse Spark models)
		providerNPM := mdl.ProviderNPM
		if providerNPM == "" && strings.Contains(strings.ToLower(mdl.ID), "muse") {
			providerNPM = "@ai-sdk/openai"
		}
		if providerNPM != "" {
			mEntry["provider"] = map[string]interface{}{
				"npm": providerNPM,
			}
		}

		modelsDict[mdl.ID] = mEntry
	}
	return modelsDict
}

