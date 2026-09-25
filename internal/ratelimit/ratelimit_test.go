package ratelimit

import (
	"net/http"
	"testing"
	"time"
)

func TestParseResetDuration(t *testing.T) {
	// Test Retry-After header with seconds
	header := http.Header{}
	header.Set("Retry-After", "120")
	d := ParseResetDuration(header, "")
	if d != 120*time.Second {
		t.Fatalf("expected 120s, got %v", d)
	}

	// Test body text with "resets in 2 hours 15 minutes"
	body := "Rate limit reached. resets in 2 hours 15 minutes"
	d2 := ParseResetDuration(nil, body)
	if d2 != (2*time.Hour + 15*time.Minute) {
		t.Fatalf("expected 2h15m, got %v", d2)
	}

	// Test body text with "resets in 45 seconds"
	body2 := "resets in 45 seconds"
	d3 := ParseResetDuration(nil, body2)
	if d3 != 45*time.Second {
		t.Fatalf("expected 45s, got %v", d3)
	}

	// Fallback default
	dDefault := ParseResetDuration(nil, "something without time")
	if dDefault != 5*time.Minute {
		t.Fatalf("expected default 5m, got %v", dDefault)
	}
}
