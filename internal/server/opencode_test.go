package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aip/internal/config"
	"aip/internal/db"
	"aip/internal/opencode"
)

func setupTestServer(t *testing.T) (*Server, string) {
	tempDir, err := os.MkdirTemp("", "aip-server-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	openCodeJSON := filepath.Join(tempDir, "opencode.json")
	cfg := &config.Config{
		DataDir:      tempDir,
		DBPath:       dbPath,
		OpenCodeJSON: openCodeJSON,
		Port:         9100,
	}

	s := &Server{
		cfg:       cfg,
		db:        database,
		opencode:  opencode.NewManager(openCodeJSON),
		logger:    NewEventLogger(10),
		startTime: time.Now(),
	}

	return s, tempDir
}

func TestIntegrationsStandardREST(t *testing.T) {
	s, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	// Seed a test model in DB
	err := s.db.UpdateModels([]*db.Model{
		{
			ID:         "test-model-1",
			Name:       "Test Model 1",
			Modalities: "text",
		},
	})
	if err != nil {
		t.Fatalf("failed to seed model: %v", err)
	}

	// 1. GET /api/integrations initially (unlinked, file doesn't exist)
	{
		req := httptest.NewRequest(http.MethodGet, "/api/integrations", nil)
		rec := httptest.NewRecorder()
		s.handleIntegrations(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var st opencode.StatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&st); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if st.IsLinked {
			t.Errorf("expected is_linked = false, got true")
		}
		if st.ConfigFileExists {
			t.Errorf("expected config_file_exists = false, got true")
		}
	}

	// 2. PUT /api/integrations (link and sync models)
	{
		req := httptest.NewRequest(http.MethodPut, "/api/integrations", nil)
		rec := httptest.NewRecorder()
		s.handleIntegrations(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var st opencode.StatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&st); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !st.IsLinked {
			t.Errorf("expected is_linked = true, got false")
		}
		if !st.ConfigFileExists {
			t.Errorf("expected config_file_exists = true, got false")
		}
		if len(st.SyncedModels) != 1 || st.SyncedModels[0] != "test-model-1" {
			t.Errorf("expected synced_models ['test-model-1'], got %v", st.SyncedModels)
		}
	}

	// 3. DELETE /api/integrations (unlink)
	{
		req := httptest.NewRequest(http.MethodDelete, "/api/integrations", nil)
		rec := httptest.NewRecorder()
		s.handleIntegrations(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var st opencode.StatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&st); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if st.IsLinked {
			t.Errorf("expected is_linked = false after delete, got true")
		}
		if len(st.SyncedModels) != 0 {
			t.Errorf("expected 0 synced models, got %v", st.SyncedModels)
		}
	}

	// 4. Invalid Method on /api/integrations
	{
		req := httptest.NewRequest(http.MethodPost, "/api/integrations", nil)
		rec := httptest.NewRecorder()
		s.handleIntegrations(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 Method Not Allowed, got %d", rec.Code)
		}
	}
}

func TestModelsStandardREST(t *testing.T) {
	s, tempDir := setupTestServer(t)
	defer os.RemoveAll(tempDir)

	// Seed test models in DB
	err := s.db.UpdateModels([]*db.Model{
		{
			ID:         "test-model-1",
			Name:       "Test Model 1",
			Modalities: "text",
		},
	})
	if err != nil {
		t.Fatalf("failed to seed model: %v", err)
	}

	// 1. GET /api/models
	{
		req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
		rec := httptest.NewRecorder()
		s.handleModels(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var list []*db.Model
		if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(list) != 1 || list[0].ID != "test-model-1" {
			t.Errorf("expected 1 model 'test-model-1', got %v", list)
		}
	}

	// 2. Invalid method on /api/models (e.g. POST or DELETE)
	{
		req := httptest.NewRequest(http.MethodPost, "/api/models", nil)
		rec := httptest.NewRecorder()
		s.handleModels(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 Method Not Allowed, got %d", rec.Code)
		}
	}
}

