package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"
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

	// OpenAI Compatible Routes (gateway.go)
	mux.HandleFunc("/v1/models", s.handleV1Models)
	mux.HandleFunc("/v1/", s.handleV1Passthrough)

	// Dashboard REST API (dashboard.go)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/usage/summary", s.handleUsageSummary)
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
