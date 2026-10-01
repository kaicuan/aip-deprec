package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aip/internal/db"
)

const catalogURL = "https://models.dev/catalog.json"

type CatalogResponse struct {
	Providers struct {
		OpenCode ProviderEntry `json:"opencode"`
	} `json:"providers"`
	Models map[string]json.RawMessage `json:"models"`
}

type ProviderEntry struct {
	ID     string                  `json:"id"`
	Name   string                  `json:"name"`
	API    string                  `json:"api"`
	Models map[string]CatalogModel `json:"models"`
}

type CatalogModel struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Family     string          `json:"family,omitempty"`
	Status     string          `json:"status"`
	Limit      json.RawMessage `json:"limit"`
	Modalities struct {
		Input []string `json:"input"`
	} `json:"modalities"`
	Cost             ModelCost         `json:"cost"`
	Reasoning        bool              `json:"reasoning"`
	ReasoningOptions []ReasoningOption `json:"reasoning_options"`
	ToolCall         *bool             `json:"tool_call"`
	Temperature      *bool             `json:"temperature"`
	Attachment       *bool             `json:"attachment"`
	Provider         json.RawMessage   `json:"provider"`
}

type ModelCost struct {
	Input     float64 `json:"input"`
	Output    float64 `json:"output"`
	CacheRead float64 `json:"cache_read"`
}

type ReasoningOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
}

type DiscoveryService struct {
	db *db.DB
}

func NewDiscoveryService(database *db.DB) *DiscoveryService {
	return &DiscoveryService{db: database}
}

func (s *DiscoveryService) FetchFreeModels() ([]*db.Model, error) {
	// Fetch models.dev catalog
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(catalogURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch model catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, catalogURL)
	}

	var catResp CatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&catResp); err != nil {
		return nil, fmt.Errorf("failed to decode catalog from %s: %w", catalogURL, err)
	}

	opencodeEntry := catResp.Providers.OpenCode
	if len(opencodeEntry.Models) == 0 {
		return nil, fmt.Errorf("opencode provider empty or missing from catalog")
	}

	// Index canonical models for creator lab resolution
	nameToLab := make(map[string]string, len(catResp.Models))
	familyToLab := make(map[string]string)
	for key, raw := range catResp.Models {
		parts := strings.Split(key, "/")
		if len(parts) == 2 {
			nameToLab[strings.ToLower(parts[1])] = parts[0]

			var meta struct {
				Family string `json:"family"`
			}
			if err := json.Unmarshal(raw, &meta); err == nil && meta.Family != "" {
				familyToLab[strings.ToLower(meta.Family)] = parts[0]
			}
		}
	}

	// Index paid models for commercial rate lookup
	paidRates := make(map[string]ModelCost, len(opencodeEntry.Models))
	for mid, m := range opencodeEntry.Models {
		if m.Cost.Input > 0 || m.Cost.Output > 0 {
			cacheRead := m.Cost.CacheRead
			if cacheRead == 0 && m.Cost.Input > 0 {
				cacheRead = m.Cost.Input * 0.1
			}
			paidRates[normalizeModelID(mid)] = ModelCost{
				Input:     m.Cost.Input,
				Output:    m.Cost.Output,
				CacheRead: cacheRead,
			}
		}
	}

	// Filter and construct active free models
	now := time.Now().Unix()
	var savedModels []*db.Model

	for modelID, m := range opencodeEntry.Models {
		if strings.ToLower(m.Status) == "deprecated" || m.Cost.Input != 0 || m.Cost.Output != 0 {
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

		cleanID := normalizeModelID(modelID)
		lab := resolveLab(cleanID, m.Family, nameToLab, familyToLab)

		commercialRateJSON := "{}"
		if rate, ok := paidRates[cleanID]; ok {
			b, _ := json.Marshal(rate)
			commercialRateJSON = string(b)
		}

		limitJSON := string(m.Limit)
		if limitJSON == "" || limitJSON == "null" {
			limitJSON = "{}"
		}

		reasoningOptionsJSON := ""
		if len(m.ReasoningOptions) > 0 {
			b, _ := json.Marshal(m.ReasoningOptions)
			reasoningOptionsJSON = string(b)
		}

		providerJSON := string(m.Provider)
		if providerJSON == "null" || providerJSON == "{}" {
			providerJSON = ""
		}
		if providerJSON == "" && strings.Contains(strings.ToLower(modelID), "muse") {
			providerJSON = `{"npm":"@ai-sdk/openai"}`
		}

		temp := false
		if m.Temperature != nil {
			temp = *m.Temperature
		}
		toolCall := false
		if m.ToolCall != nil {
			toolCall = *m.ToolCall
		}
		attachment := false
		if m.Attachment != nil {
			attachment = *m.Attachment
		}

		savedModels = append(savedModels, &db.Model{
			ID:               modelID,
			Name:             name,
			Limit:            limitJSON,
			Modalities:       strings.Join(modalitiesList, ","),
			Reasoning:        m.Reasoning,
			ToolCall:         toolCall,
			Temperature:      temp,
			Attachment:       attachment,
			ReasoningOptions: reasoningOptionsJSON,
			Provider:         providerJSON,
			Lab:              lab,
			CommercialRate:   commercialRateJSON,
			FetchedAt:        now,
		})
	}

	// Atomically persist models to database
	if err := s.db.UpdateModels(savedModels); err != nil {
		return nil, fmt.Errorf("failed to update models: %w", err)
	}

	return savedModels, nil
}

func resolveLab(cleanID, family string, nameToLab, familyToLab map[string]string) string {
	if lab, ok := nameToLab[cleanID]; ok {
		return lab
	}
	if family != "" {
		famLower := strings.ToLower(family)
		if lab, ok := familyToLab[famLower]; ok {
			return lab
		}
		if lab, ok := familyToLab[normalizeModelID(famLower)]; ok {
			return lab
		}
	}
	if idx := strings.IndexByte(cleanID, '-'); idx > 0 {
		prefix := cleanID[:idx]
		if lab, ok := familyToLab[prefix]; ok {
			return lab
		}
		return prefix
	}
	return "opencode"
}

func normalizeModelID(modelID string) string {
	clean := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(modelID, "-free"), "-contributor"), "-fin")
	return strings.ToLower(clean)
}
