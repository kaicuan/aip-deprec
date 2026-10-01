package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestGetUsageSummary(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer database.Close()

	now := time.Now().UTC()
	// Insert dummy logs
	log1 := &RequestLog{
		Timestamp:        now.Unix(),
		Model:            "gemini-2.5-flash",
		ProxyURL:         "socks5://proxy1:1080",
		StatusCode:       200,
		DurationMs:       450,
		PromptTokens:     100,
		CompletionTokens: 50,
		CacheReadTokens:  20,
		EstimatedSavings: 0.05,
	}
	log2 := &RequestLog{
		Timestamp:        now.Unix(),
		Model:            "deepseek-chat",
		ProxyURL:         "socks5://proxy2:1080",
		StatusCode:       429,
		DurationMs:       120,
		PromptTokens:     80,
		CompletionTokens: 0,
		CacheReadTokens:  0,
		EstimatedSavings: 0.0,
	}
	if err := database.InsertRequestLog(log1); err != nil {
		t.Fatalf("failed to insert log1: %v", err)
	}
	if err := database.InsertRequestLog(log2); err != nil {
		t.Fatalf("failed to insert log2: %v", err)
	}

	for _, r := range []string{"today", "weekly", "monthly", "24h", "7d", "30d"} {
		s, err := database.GetUsageSummary(r)
		if err != nil {
			t.Fatalf("GetUsageSummary(%s) failed: %v", r, err)
		}
		if s.TotalRequests != 2 {
			t.Errorf("expected 2 requests for range %s, got %d", r, s.TotalRequests)
		}
		if s.TotalTokens != 230 {
			t.Errorf("expected 230 tokens for range %s, got %d", r, s.TotalTokens)
		}
		if len(s.TimeBuckets) == 0 {
			t.Errorf("expected time buckets for range %s, got 0", r)
		}
		if (r == "today" || r == "24h") && len(s.TimeBuckets) != 12 {
			t.Errorf("expected 12 time buckets for range %s, got %d", r, len(s.TimeBuckets))
		}
		if (r == "weekly" || r == "7d") && len(s.TimeBuckets) != 14 {
			t.Errorf("expected 14 time buckets for range %s, got %d", r, len(s.TimeBuckets))
		}
		if s.ModelUsage["gemini-2.5-flash"] != 1 {
			t.Errorf("expected 1 gemini request, got %d", s.ModelUsage["gemini-2.5-flash"])
		}
		if s.ModelTokens["gemini-2.5-flash"] != 150 {
			t.Errorf("expected 150 gemini tokens, got %d", s.ModelTokens["gemini-2.5-flash"])
		}
		if s.ProxyUsage["socks5://proxy1:1080"] != 1 {
			t.Errorf("expected 1 proxy1 request, got %d", s.ProxyUsage["socks5://proxy1:1080"])
		}
	}
}
