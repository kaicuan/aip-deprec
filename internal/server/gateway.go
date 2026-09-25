package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// /v1/models
func (s *Server) handleV1Models(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	models, err := s.db.ListModels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type modelObj struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	data := make([]modelObj, 0, len(models))
	for _, m := range models {
		created := m.FetchedAt
		if created == 0 {
			created = time.Now().Unix()
		}
		data = append(data, modelObj{
			ID:      m.ID,
			Object:  "model",
			Created: created,
			OwnedBy: m.Provider,
		})
	}

	resp := map[string]interface{}{
		"object": "list",
		"data":   data,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// /v1/*
func (s *Server) handleV1Passthrough(w http.ResponseWriter, r *http.Request) {
	pathSuffix := strings.TrimPrefix(r.URL.Path, "/v1/")
	s.upstream.Forward(w, r, pathSuffix)
}
