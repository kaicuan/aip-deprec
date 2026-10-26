package upstream

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aip/internal/db"
	"aip/internal/proxy"
)

func TestCopyHeaderWithoutHopByHop(t *testing.T) {
	src := http.Header{
		"Authorization":     []string{"Bearer secret"},
		"Content-Type":      []string{"application/json"},
		"Transfer-Encoding": []string{"chunked"},
		"Connection":        []string{"keep-alive"},
		"X-Custom-Header":   []string{"value1", "value2"},
	}

	dst := make(http.Header)
	copyHeaderWithoutHopByHop(dst, src)

	if dst.Get("Authorization") != "Bearer secret" {
		t.Errorf("expected Authorization to be preserved, got %q", dst.Get("Authorization"))
	}
	if dst.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type to be preserved, got %q", dst.Get("Content-Type"))
	}
	if dst.Get("Transfer-Encoding") != "" {
		t.Errorf("expected Transfer-Encoding to be omitted, got %q", dst.Get("Transfer-Encoding"))
	}
	if dst.Get("Connection") != "" {
		t.Errorf("expected Connection to be omitted, got %q", dst.Get("Connection"))
	}
	if len(dst["X-Custom-Header"]) != 2 {
		t.Errorf("expected 2 values for X-Custom-Header, got %d", len(dst["X-Custom-Header"]))
	}
}

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name      string
		primary   string
		secondary string
		expected  int
	}{
		{
			name:     "Found in primary stream tail",
			primary:  `data: {"usage":{"prompt_tokens":42,"completion_tokens":128}}`,
			expected: 42,
		},
		{
			name:      "Found in secondary buffer when missing in primary",
			primary:   "data: [DONE]",
			secondary: `{"prompt_tokens":99}`,
			expected:  99,
		},
		{
			name:     "OpenAI / Anthropic alternative naming (input_tokens)",
			primary:  `{"input_tokens": 512}`,
			expected: 512,
		},
		{
			name:     "No token match returns 0",
			primary:  `{"choices":[]}`,
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractToken(promptTokensRegex, tc.primary, tc.secondary)
			if got != tc.expected {
				t.Errorf("extractToken() = %d, want %d", got, tc.expected)
			}
		})
	}

	chunk := `data: {"usage":{"prompt_tokens":17274,"completion_tokens":512,"total_tokens":17786,"prompt_tokens_details":{"cached_tokens":1912},"completion_tokens_details":{"reasoning_tokens":249}}}`
	if got := extractToken(cachedTokensRegex, chunk, ""); got != 1912 {
		t.Errorf("extractToken(cachedTokensRegex) = %d, want 1912", got)
	}
	if got := extractToken(reasoningTokensRegex, chunk, ""); got != 249 {
		t.Errorf("extractToken(reasoningTokensRegex) = %d, want 249", got)
	}
}

func TestForwardExhaustsAllPoolProxies(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&attempts, 1)
		if cur < 5 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	database, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer database.Close()

	pool := proxy.NewPool(database)

	for i := 1; i <= 5; i++ {
		p := &db.Proxy{
			RawInput:  fmt.Sprintf("http://127.0.0.1:808%d", i),
			URL:       fmt.Sprintf("http://127.0.0.1:808%d", i),
			Protocol:  "direct",
			Host:      "localhost",
			Status:    "online",
			CreatedAt: 1000,
		}
		if err := database.InsertProxy(p); err != nil {
			t.Fatalf("failed to insert proxy: %v", err)
		}
	}

	u := NewUpstream(database, pool)
	u.baseURL = ts.URL

	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/v1/chat/completions", strings.NewReader(`{"model":"test"}`))
	rec := httptest.NewRecorder()

	u.Forward(rec, req, "chat/completions")

	if atomic.LoadInt32(&attempts) != 5 {
		t.Errorf("expected 5 attempts across the pool, got %d", attempts)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestForwardExhaustsAllPoolProxies_AllFail429(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	database, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer database.Close()

	pool := proxy.NewPool(database)

	for i := 1; i <= 5; i++ {
		p := &db.Proxy{
			RawInput:  fmt.Sprintf("http://127.0.0.1:808%d", i),
			URL:       fmt.Sprintf("http://127.0.0.1:808%d", i),
			Protocol:  "direct",
			Host:      "localhost",
			Status:    "online",
			CreatedAt: 1000,
		}
		if err := database.InsertProxy(p); err != nil {
			t.Fatalf("failed to insert proxy: %v", err)
		}
	}

	u := NewUpstream(database, pool)
	u.baseURL = ts.URL

	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/v1/chat/completions", strings.NewReader(`{"model":"test"}`))
	rec := httptest.NewRecorder()

	u.Forward(rec, req, "chat/completions")

	if atomic.LoadInt32(&attempts) != 5 {
		t.Errorf("expected 5 attempts across the pool, got %d", attempts)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

func TestForwardThroughRelay(t *testing.T) {
	var receivedTarget, receivedPath, receivedAuth, receivedHost string
	relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTarget = r.Header.Get("x-relay-target")
		receivedPath = r.Header.Get("x-relay-path")
		receivedAuth = r.Header.Get("Authorization")
		receivedHost = r.Host
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test"}`))
	}))
	defer relayServer.Close()

	tempDir := t.TempDir()
	database, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer database.Close()

	pool := proxy.NewPool(database)

	p := &db.Proxy{
		RawInput:  relayServer.URL,
		URL:       relayServer.URL,
		Protocol:  "relay",
		Host:      relayServer.Listener.Addr().String(),
		Status:    "online",
		CreatedAt: 1000,
	}
	if err := database.InsertProxy(p); err != nil {
		t.Fatalf("failed to insert relay proxy: %v", err)
	}

	u := NewUpstream(database, pool)
	u.baseURL = "https://opencode.ai/inference/openai/v1"

	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/v1/chat/completions", strings.NewReader(`{"model":"test"}`))
	rec := httptest.NewRecorder()

	u.Forward(rec, req, "chat/completions")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if receivedTarget != "https://opencode.ai" {
		t.Errorf("expected x-relay-target https://opencode.ai, got %q", receivedTarget)
	}
	if receivedPath != "/inference/openai/v1/chat/completions" {
		t.Errorf("expected x-relay-path /inference/openai/v1/chat/completions, got %q", receivedPath)
	}
	if receivedAuth != "Bearer public" {
		t.Errorf("expected Authorization Bearer public, got %q", receivedAuth)
	}
	if receivedHost != relayServer.Listener.Addr().String() {
		t.Errorf("expected Host header %q, got %q", relayServer.Listener.Addr().String(), receivedHost)
	}
}

