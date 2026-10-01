package upstream

import (
	"net/http"
	"testing"
	"time"
)

func TestParseResetDuration(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		body     string
		expected time.Duration
	}{
		{
			name: "Retry-After header with integer seconds",
			headers: func() http.Header {
				h := make(http.Header)
				h.Set("Retry-After", "120")
				return h
			}(),
			expected: 120 * time.Second,
		},
		{
			name: "x-ratelimit-reset relative seconds",
			headers: func() http.Header {
				h := make(http.Header)
				h.Set("x-ratelimit-reset", "60")
				return h
			}(),
			expected: 60 * time.Second,
		},
		{
			name:     "Body text with hours and minutes",
			body:     "Rate limit reached. resets in 2 hours 15 minutes",
			expected: 2*time.Hour + 15*time.Minute,
		},
		{
			name:     "Body text with short units (1h 30m)",
			body:     "try again in 1h 30m",
			expected: 1*time.Hour + 30*time.Minute,
		},
		{
			name:     "Body text with seconds only",
			body:     "resets in 45 seconds",
			expected: 45 * time.Second,
		},
		{
			name:     "Body text with minutes only",
			body:     "resets in 10m",
			expected: 10 * time.Minute,
		},
		{
			name:     "Fallback default duration when unspecified",
			body:     "An unexpected rate limit has occurred",
			expected: 5 * time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseResetDuration(tc.headers, tc.body)
			if got != tc.expected {
				t.Errorf("ParseResetDuration() = %v, want %v", got, tc.expected)
			}
		})
	}
}
