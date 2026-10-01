package upstream

import (
	"net/http"
	"testing"
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
