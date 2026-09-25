package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aip/internal/config"
	"aip/internal/db"
	"aip/internal/discovery"
	"aip/internal/opencode"
	"aip/internal/proxy"
	"aip/internal/upstream"
	"aip/web"
)

type Server struct {
	cfg       *config.Config
	db        *db.DB
	pool      *proxy.Pool
	upstream  *upstream.Upstream
	discovery *discovery.DiscoveryService
	opencode  *opencode.Manager
	logger    *EventLogger
	startTime time.Time
	httpSrv   *http.Server
}

func NewServer(cfg *config.Config, database *db.DB) *Server {
	pool := proxy.NewPool(database)
	up := upstream.NewUpstream(database, pool)
	disc := discovery.NewDiscoveryService(database)
	oc := opencode.NewManager(cfg.OpenCodeJSON)
	logger := NewEventLogger(500)

	s := &Server{
		cfg:       cfg,
		db:        database,
		pool:      pool,
		upstream:  up,
		discovery: disc,
		opencode:  oc,
		logger:    logger,
		startTime: time.Now(),
	}

	return s
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// OpenAI Compatible Routes
	mux.HandleFunc("/v1/models", s.handleV1Models)
	mux.HandleFunc("/v1/", s.handleV1Passthrough)

	// Dashboard REST API
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/proxies", s.handleProxies)
	mux.HandleFunc("/api/proxies/test", s.handleProxiesTest)
	mux.HandleFunc("/api/proxies/", s.handleSingleProxy)
	mux.HandleFunc("/api/models", s.handleModels)
	mux.HandleFunc("/api/models/fetch", s.handleModelsFetch)
	mux.HandleFunc("/api/opencode/status", s.handleOpenCodeStatus)
	mux.HandleFunc("/api/opencode/link", s.handleOpenCodeLink)
	mux.HandleFunc("/api/opencode/unlink", s.handleOpenCodeUnlink)
	mux.HandleFunc("/api/usage", s.handleUsage)
	mux.HandleFunc("/api/usage/", s.handleSingleUsage)
	mux.HandleFunc("/api/logs", s.logger.HandleSSE)

	// Static SPA File Server
	webFS := web.GetFS()
	fileServer := http.FileServer(http.FS(webFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If it's an API route or V1 route not found, 404
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") {
			http.NotFound(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(r.URL.Path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// Try opening the requested file in static FS
		f, err := webFS.Open(cleanPath)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Otherwise serve index.html for SPA router
		indexFile, err := webFS.Open("index.html")
		if err != nil {
			http.Error(w, "Web UI not found", http.StatusNotFound)
			return
		}
		defer indexFile.Close()

		stat, _ := indexFile.Stat()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if stat != nil {
			http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(io.ReadSeeker))
		} else {
			_, _ = io.Copy(w, indexFile)
		}
	})

	handler := s.corsMiddleware(mux)

	s.httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: handler,
	}

	s.logger.Info("daemon", "AIP Gateway starting on http://localhost:%d", s.cfg.Port)
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown() error {
	if s.httpSrv != nil {
		s.logger.Info("daemon", "AIP Gateway shutting down")
		return s.httpSrv.Close()
	}
	return nil
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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

// /api/health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime).Round(time.Second).String()
	stats, _ := s.db.GetStats()

	activeCount := int64(0)
	totalReqs := int64(0)
	successRate := 100.0

	if stats != nil {
		activeCount = stats.ActiveProxiesCount
		totalReqs = stats.TotalRequests
		successRate = stats.SuccessRate
	}

	resp := map[string]interface{}{
		"status":         "ok",
		"uptime":         uptime,
		"active_proxies": activeCount,
		"total_requests": totalReqs,
		"success_rate":   successRate,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// /api/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
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

// /api/models (GET)
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
}

// /api/models/fetch (POST)
func (s *Server) handleModelsFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logger.Info("discovery", "Fetching active free models from models.dev...")
	models, err := s.discovery.FetchFreeModels()
	if err != nil {
		s.logger.Error("discovery", "Failed to fetch models: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.logger.Info("discovery", "Discovered %d active free models", len(models))

	// Auto sync if linked
	autoSynced, syncErr := s.opencode.AutoSyncIfLinked(models)
	if syncErr != nil {
		s.logger.Warn("sync", "Auto-sync to opencode.json failed: %v", syncErr)
	} else if autoSynced {
		s.logger.Info("sync", "Automatically updated opencode.json with %d free models", len(models))
	}

	resp := map[string]interface{}{
		"count":       len(models),
		"models":      models,
		"auto_synced": autoSynced,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// /api/opencode/status (GET)
func (s *Server) handleOpenCodeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	st, err := s.opencode.CheckStatus()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	models, _ := s.db.ListModels()
	snippet := opencode.GenerateSnippet(models)

	resp := map[string]interface{}{
		"status":  st,
		"snippet": snippet,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// /api/opencode/link (POST)
func (s *Server) handleOpenCodeLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	models, err := s.db.ListModels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.opencode.LinkOrSync(models); err != nil {
		s.logger.Error("sync", "Failed to link OpenCode: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.logger.Info("sync", "Successfully linked AIP to OpenCode configuration with %d models", len(models))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "synced_count": len(models)})
}

// /api/opencode/unlink (POST)
func (s *Server) handleOpenCodeUnlink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.opencode.Unlink(); err != nil {
		s.logger.Error("sync", "Failed to unlink OpenCode: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.logger.Info("sync", "Unlinked AIP provider from OpenCode configuration")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
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
