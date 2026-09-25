package proxy

import (
	"path/filepath"
	"testing"

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
