package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aip/internal/db"
)

type ReasoningOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
}

type CatalogModel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Limit      struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
	Modalities struct {
		Input  []string `json:"input"`
		Output []string `json:"output"`
	} `json:"modalities"`
	Cost struct {
		Input  float64 `json:"input"`
		Output float64 `json:"output"`
	} `json:"cost"`
	Reasoning        bool                   `json:"reasoning"`
	ReasoningOptions []ReasoningOption      `json:"reasoning_options"`
	Variants         map[string]interface{} `json:"variants"`
	Temperature      *bool                  `json:"temperature"`
	ToolCall         *bool                  `json:"tool_call"`
	Attachment       *bool                  `json:"attachment"`
	Provider         *struct {
		NPM string `json:"npm"`
		API string `json:"api"`
	} `json:"provider"`
}

type ProviderEntry struct {
	ID     string                  `json:"id"`
	Name   string                  `json:"name"`
	API    string                  `json:"api"`
	Models map[string]CatalogModel `json:"models"`
}

type CatalogResponse struct {
	Providers map[string]ProviderEntry `json:"providers"`
	Models    map[string]interface{}   `json:"models"`
}

type DiscoveryService struct {
	db *db.DB
}

func NewDiscoveryService(database *db.DB) *DiscoveryService {
	return &DiscoveryService{db: database}
}

// resolveLab dynamically finds the canonical creator lab from the models.dev catalog
func resolveLab(modelID string, canonicalModels map[string]interface{}) string {
	// If ID already contains a creator/lab prefix (e.g. "openai/gpt-4o"), use it
	if parts := strings.Split(modelID, "/"); len(parts) == 2 && parts[0] != "opencode" {
		return parts[0]
	}

	clean := strings.TrimSuffix(strings.TrimSuffix(modelID, "-free"), "-contributor")
	cleanLower := strings.ToLower(clean)

	for canonicalKey := range canonicalModels {
		parts := strings.Split(canonicalKey, "/")
		if len(parts) != 2 {
			continue
		}
		lab := parts[0]
		canonicalName := strings.ToLower(parts[1])

		if cleanLower == canonicalName ||
			strings.HasPrefix(cleanLower, canonicalName) ||
			strings.HasPrefix(canonicalName, cleanLower) {
			return lab
		}
	}

	return "opencode"
}

func (s *DiscoveryService) FetchFreeModels() ([]*db.Model, error) {
	urls := []string{
		"https://models.dev/catalog.json",
		"https://models.opencode.ai/catalog.json",
		"https://models.dev/api.json",
		"https://models.opencode.ai/api.json",
	}

	var data map[string]ProviderEntry
	var canonicalModels map[string]interface{}
	var lastErr error

	client := &http.Client{Timeout: 15 * time.Second}

	for _, u := range urls {
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "aip-gateway/1.0")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d from %s", resp.StatusCode, u)
			continue
		}

		var catResp CatalogResponse
		if err := json.Unmarshal(body, &catResp); err == nil && len(catResp.Providers) > 0 {
			data = catResp.Providers
			canonicalModels = catResp.Models
		} else {
			if err := json.Unmarshal(body, &data); err != nil {
				lastErr = err
				continue
			}
		}

		// Successfully retrieved
		lastErr = nil
		break
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to fetch model catalog: %w", lastErr)
	}

	opencodeEntry, exists := data["opencode"]
	if !exists {
		return nil, fmt.Errorf("opencode provider not found in catalog")
	}

	now := time.Now().Unix()
	var savedModels []*db.Model

	for modelID, m := range opencodeEntry.Models {
		status := strings.ToLower(m.Status)
		if status == "deprecated" {
			continue
		}

		// Must be free: cost.input == 0 && cost.output == 0
		if m.Cost.Input != 0 || m.Cost.Output != 0 {
			continue
		}

		name := m.Name
		if name == "" {
			name = modelID
		}

		modalitiesList := m.Modalities.Input
		if len(modalitiesList) == 0 {
			modalitiesList = []string{"text"}
		}
		modalitiesStr := strings.Join(modalitiesList, ",")

		// Derive variants 1:1 with OpenCode
		variantsMap := make(map[string]interface{})
		if len(m.Variants) > 0 {
			variantsMap = m.Variants
		} else {
			var effortValues []string
			for _, ro := range m.ReasoningOptions {
				if ro.Type == "effort" && len(ro.Values) > 0 {
					effortValues = ro.Values
					break
				}
			}

			if len(effortValues) == 0 && m.Reasoning {
				idLower := strings.ToLower(modelID)
				nonEffortModels := []string{"big-pickle", "nemotron", "mimo", "minimax", "qwen", "glm", "kimi"}
				hasNoEffort := false
				for _, ne := range nonEffortModels {
					if strings.Contains(idLower, ne) {
						hasNoEffort = true
						break
					}
				}
				if !hasNoEffort {
					effortValues = []string{"low", "medium", "high"}
				}
			}

			for _, v := range effortValues {
				variantsMap[v] = map[string]string{
					"reasoningEffort": v,
				}
			}
		}

		variantsJSON, _ := json.Marshal(variantsMap)

		// Derive capabilities 1:1 with OpenCode
		capsMap := make(map[string]interface{})
		if m.Reasoning {
			capsMap["reasoning"] = true
		}
		if m.Temperature != nil {
			capsMap["temperature"] = *m.Temperature
		}
		if m.ToolCall != nil {
			capsMap["tool_call"] = *m.ToolCall
		}
		if m.Attachment != nil {
			capsMap["attachment"] = *m.Attachment
		}

		capsJSON, _ := json.Marshal(capsMap)

		lab := resolveLab(modelID, canonicalModels)

		var providerNPM string
		if m.Provider != nil && m.Provider.NPM != "" {
			providerNPM = m.Provider.NPM
		} else if strings.Contains(strings.ToLower(modelID), "muse") {
			// Muse models require OpenAI Responses API SDK adapter
			providerNPM = "@ai-sdk/openai"
		}

		modelRecord := &db.Model{
			ID:            modelID,
			Name:          name,
			Provider:      "opencode",
			Lab:           lab,
			ContextLength: m.Limit.Context,
			OutputLimit:   m.Limit.Output,
			Modalities:    modalitiesStr,
			IsFree:        1,
			FetchedAt:     now,
			Variants:      string(variantsJSON),
			Capabilities:  string(capsJSON),
			ProviderNPM:   providerNPM,
		}

		savedModels = append(savedModels, modelRecord)
	}

	if err := s.db.UpdateModels(savedModels); err != nil {
		return nil, fmt.Errorf("failed to update models: %w", err)
	}

	return savedModels, nil
}
