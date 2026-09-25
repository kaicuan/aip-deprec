package proxy

import (
	"testing"
)

func TestParseProxy(t *testing.T) {
	// Direct
	p, err := ParseProxy("direct", "http")
	if err != nil || p.Protocol != "direct" {
		t.Fatalf("failed direct: %v", err)
	}

	// Vendor format: host:port:username:password
	p2, err := ParseProxy("31.59.20.176:6754:glzkmuga:qg5wcyihw7os", "http")
	if err != nil {
		t.Fatalf("failed vendor: %v", err)
	}
	if p2.Host != "31.59.20.176" || p2.Port != 6754 || p2.Username != "glzkmuga" || p2.Password != "qg5wcyihw7os" {
		t.Fatalf("unexpected fields in vendor proxy: %+v", p2)
	}
	if p2.Protocol != "http" {
		t.Fatalf("expected http protocol, got %s", p2.Protocol)
	}

	// Standard URL
	p3, err := ParseProxy("socks5://user:pass@127.0.0.1:1080", "http")
	if err != nil {
		t.Fatalf("failed url: %v", err)
	}
	if p3.Protocol != "socks5" || p3.Host != "127.0.0.1" || p3.Port != 1080 || p3.Username != "user" || p3.Password != "pass" {
		t.Fatalf("unexpected fields in standard socks5 proxy: %+v", p3)
	}
}
