package proxy

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"aip/internal/db"
)

func TestParseProxy(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		defaultProto string
		wantErr      bool
		wantProto    string
		wantHost     string
		wantPort     int
		wantUser     string
		wantPass     string
	}{
		{
			name:         "Direct connection",
			raw:          "direct",
			defaultProto: "http",
			wantProto:    "direct",
			wantHost:     "localhost",
		},
		{
			name:         "Vendor format host:port:user:pass",
			raw:          "192.168.1.100:8080:myuser:mypassword",
			defaultProto: "http",
			wantProto:    "http",
			wantHost:     "192.168.1.100",
			wantPort:     8080,
			wantUser:     "myuser",
			wantPass:     "mypassword",
		},
		{
			name:         "Standard SOCKS5 URL",
			raw:          "socks5://alice:secret@10.0.0.1:1080",
			defaultProto: "http",
			wantProto:    "socks5",
			wantHost:     "10.0.0.1",
			wantPort:     1080,
			wantUser:     "alice",
			wantPass:     "secret",
		},
		{
			name:         "Standard HTTP URL without auth",
			raw:          "http://proxy.example.com:3128",
			defaultProto: "http",
			wantProto:    "http",
			wantHost:     "proxy.example.com",
			wantPort:     3128,
		},
		{
			name:         "Empty string input errors",
			raw:          "",
			defaultProto: "http",
			wantErr:      true,
		},
		{
			name:         "Unsupported protocol errors",
			raw:          "ftp://user:pass@1.2.3.4:21",
			defaultProto: "http",
			wantErr:      true,
		},
		{
			name:         "Explicit relay protocol scheme",
			raw:          "relay://my-worker.workers.dev",
			defaultProto: "http",
			wantProto:    "relay",
			wantHost:     "my-worker.workers.dev",
			wantPort:     443,
		},
		{
			name:         "Auto-detect Cloudflare workers.dev as relay",
			raw:          "https://cloudflare-relay1.anya1-linerangers.workers.dev",
			defaultProto: "http",
			wantProto:    "relay",
			wantHost:     "cloudflare-relay1.anya1-linerangers.workers.dev",
			wantPort:     443,
		},
		{
			name:         "Auto-detect Vercel vercel.app as relay",
			raw:          "https://vercel-relay1.vercel.app",
			defaultProto: "http",
			wantProto:    "relay",
			wantHost:     "vercel-relay1.vercel.app",
			wantPort:     443,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ParseProxy(tc.raw, tc.defaultProto)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Protocol != tc.wantProto {
				t.Errorf("Protocol = %q, want %q", p.Protocol, tc.wantProto)
			}
			if p.Host != tc.wantHost {
				t.Errorf("Host = %q, want %q", p.Host, tc.wantHost)
			}
			if tc.wantPort != 0 && p.Port != tc.wantPort {
				t.Errorf("Port = %d, want %d", p.Port, tc.wantPort)
			}
			if tc.wantUser != "" && p.Username != tc.wantUser {
				t.Errorf("Username = %q, want %q", p.Username, tc.wantUser)
			}
			if tc.wantPass != "" && p.Password != tc.wantPass {
				t.Errorf("Password = %q, want %q", p.Password, tc.wantPass)
			}
		})
	}
}

func TestGetTransportCaching(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer database.Close()

	pool := NewPool(database)

	p := &db.Proxy{
		URL:      "http://user:pass@127.0.0.1:8080",
		Protocol: "http",
		Host:     "127.0.0.1",
		Port:     8080,
	}

	tr1, err := pool.GetTransport(p)
	if err != nil {
		t.Fatalf("failed to get transport: %v", err)
	}

	tr2, err := pool.GetTransport(p)
	if err != nil {
		t.Fatalf("failed to get transport second time: %v", err)
	}

	if tr1 != tr2 {
		t.Errorf("expected cached transport instance to be reused (pointers %p and %p)", tr1, tr2)
	}
}

func TestRelayTestProxy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-relay-target") != "https://opencode.ai" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`missing x-relay-target`))
			return
		}
		if r.Header.Get("x-relay-path") != "/zen/v1/models" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`missing x-relay-path`))
			return
		}
		w.Header().Set("cf-ray", "12345678-SIN")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	p := &db.Proxy{
		URL:      ts.URL,
		Protocol: "relay",
		Host:     ts.Listener.Addr().String(),
	}

	lat, egress, err := TestProxy(p, 2*time.Second)
	if err != nil {
		t.Fatalf("TestProxy for relay failed: %v", err)
	}
	if lat < 0 {
		t.Errorf("expected non-negative latency, got %d", lat)
	}
	if egress != "cf-SIN" {
		t.Errorf("expected egress cf-SIN, got %q", egress)
	}
}

