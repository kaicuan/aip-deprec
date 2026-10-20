package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aip/internal/db"
	"aip/internal/proxy"
)

// /api/health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health, err := s.db.GetServerHealth()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	health.Status = "ok"
	health.Uptime = time.Since(s.startTime).Round(time.Second).String()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(health)
}

// /api/usage/summary
func (s *Server) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	rangeParam := r.URL.Query().Get("range")
	summary, err := s.db.GetUsageSummary(rangeParam)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

// /api/proxies (GET, POST, DELETE)
func (s *Server) handleProxies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		proxies, err := s.db.ListProxies()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if proxies == nil {
			proxies = []*db.Proxy{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(proxies)

	case http.MethodPost:
		var payload struct {
			Input    string `json:"input"`
			Protocol string `json:"protocol"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		lines := strings.Split(payload.Input, "\n")
		var added []*db.Proxy
		var errorsList []string

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			p, err := proxy.ParseProxy(line, payload.Protocol)
			if err != nil {
				errorsList = append(errorsList, fmt.Sprintf("%s: %v", line, err))
				continue
			}

			if err := s.db.InsertProxy(p); err != nil {
				errorsList = append(errorsList, fmt.Sprintf("%s: %v", line, err))
				continue
			}
			added = append(added, p)
		}

		s.logger.Info("proxy", "Added %d new proxies (failed: %d)", len(added), len(errorsList))
		resp := map[string]interface{}{
			"added_count": len(added),
			"added":       added,
			"errors":      errorsList,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)

	case http.MethodDelete:
		var payload struct {
			IDs []int64 `json:"ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.db.DeleteProxies(payload.IDs); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.logger.Info("proxy", "Deleted %d proxies", len(payload.IDs))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "deleted_count": len(payload.IDs)})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// /api/proxies/test (POST)
func (s *Server) handleProxiesTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		IDs []int64 `json:"ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	allProxies, err := s.db.ListProxies()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idMap := make(map[int64]bool)
	for _, id := range payload.IDs {
		idMap[id] = true
	}

	var toTest []*db.Proxy
	for _, p := range allProxies {
		if len(idMap) == 0 || idMap[p.ID] {
			toTest = append(toTest, p)
		}
	}

	s.logger.Info("proxy", "Testing %d proxies against Cloudflare CDN trace...", len(toTest))

	type testResult struct {
		ID        int64  `json:"id"`
		Status    string `json:"status"`
		LatencyMs int    `json:"latency_ms"`
		EgressIP  string `json:"egress_ip"`
		Error     string `json:"error,omitempty"`
	}

	results := make([]testResult, len(toTest))
	sem := make(chan struct{}, 10) // 10 workers
	var wg sync.WaitGroup

	for i, p := range toTest {
		wg.Add(1)
		go func(idx int, target *db.Proxy) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			latency, ip, testErr := proxy.TestProxy(target, 7*time.Second)
			status := "online"
			errStr := ""
			if testErr != nil {
				status = "error"
				errStr = testErr.Error()
			}

			_ = s.db.UpdateProxyStatus(target.ID, status, latency, ip, 0, errStr)

			results[idx] = testResult{
				ID:        target.ID,
				Status:    status,
				LatencyMs: latency,
				EgressIP:  ip,
				Error:     errStr,
			}
		}(i, p)
	}

	wg.Wait()
	s.logger.Info("proxy", "Testing finished for %d proxies", len(toTest))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

// /api/proxies/{id} (DELETE)
func (s *Server) handleSingleProxy(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/proxies/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid proxy ID", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodDelete {
		if err := s.db.DeleteProxy(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.logger.Info("proxy", "Deleted proxy ID %d", id)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// /api/models (GET, PUT)
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		models, err := s.db.ListModels()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if models == nil {
			models = []*db.Model{}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models)

	case http.MethodPut:
		s.logger.Info("discovery", "Fetching active free models from models.dev...")
		models, err := s.discovery.FetchFreeModels()
		if err != nil {
			s.logger.Error("discovery", "Failed to fetch models: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.logger.Info("discovery", "Discovered %d active free models", len(models))
		if models == nil {
			models = []*db.Model{}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// /api/integrations (GET, PUT, DELETE)
func (s *Server) handleIntegrations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st, err := s.opencode.CheckStatus()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(st)

	case http.MethodPut:
		models, err := s.db.ListModels()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := s.opencode.LinkOrSync(models); err != nil {
			s.logger.Error("sync", "Failed to link integration: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		st, err := s.opencode.CheckStatus()
		if err != nil {
			s.logger.Error("sync", "Failed to check integration status after link: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.logger.Info("sync", "Successfully linked AIP integration with %d models", len(models))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(st)

	case http.MethodDelete:
		if err := s.opencode.Unlink(); err != nil {
			s.logger.Error("sync", "Failed to unlink integration: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		st, err := s.opencode.CheckStatus()
		if err != nil {
			s.logger.Error("sync", "Failed to check integration status after unlink: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.logger.Info("sync", "Unlinked AIP integration configuration")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(st)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// /api/usage (GET)
func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	offset := 0

	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	logs, total, err := s.db.ListRequestLogs(limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if logs == nil {
		logs = []*db.RequestLog{}
	}

	resp := map[string]interface{}{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// /api/usage/{id} (GET)
func (s *Server) handleSingleUsage(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/usage/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid log ID", http.StatusBadRequest)
		return
	}

	logEntry, err := s.db.GetRequestLog(id)
	if err != nil {
		http.Error(w, "Request log not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(logEntry)
}
