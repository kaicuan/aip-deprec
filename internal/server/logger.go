package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type LogEntry struct {
	Timestamp int64  `json:"timestamp"`
	TimeStr   string `json:"time_str"`
	Level     string `json:"level"` // "info", "warn", "error"
	Source    string `json:"source"`
	Message   string `json:"message"`
}

type EventLogger struct {
	mu          sync.RWMutex
	entries     []LogEntry
	maxEntries  int
	subscribers map[chan LogEntry]struct{}
}

func NewEventLogger(maxEntries int) *EventLogger {
	if maxEntries <= 0 {
		maxEntries = 500
	}
	return &EventLogger{
		entries:     make([]LogEntry, 0, maxEntries),
		maxEntries:  maxEntries,
		subscribers: make(map[chan LogEntry]struct{}),
	}
}

func (l *EventLogger) Log(level, source, message string) {
	now := time.Now()
	entry := LogEntry{
		Timestamp: now.UnixMilli(),
		TimeStr:   now.Format("15:04:05"),
		Level:     level,
		Source:    source,
		Message:   message,
	}

	log.Printf("[%s] [%s] %s\n", level, source, message)

	l.mu.Lock()
	if len(l.entries) >= l.maxEntries {
		l.entries = l.entries[1:]
	}
	l.entries = append(l.entries, entry)

	// Broadcast to SSE clients
	for ch := range l.subscribers {
		select {
		case ch <- entry:
		default:
		}
	}
	l.mu.Unlock()
}

func (l *EventLogger) Info(source, format string, v ...interface{}) {
	l.Log("info", source, fmt.Sprintf(format, v...))
}

func (l *EventLogger) Warn(source, format string, v ...interface{}) {
	l.Log("warn", source, fmt.Sprintf(format, v...))
}

func (l *EventLogger) Error(source, format string, v ...interface{}) {
	l.Log("error", source, fmt.Sprintf(format, v...))
}

func (l *EventLogger) GetRecentEntries() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]LogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

func (l *EventLogger) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send recent logs first
	recent := l.GetRecentEntries()
	for _, entry := range recent {
		data, _ := json.Marshal(entry)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	ch := make(chan LogEntry, 30)
	l.mu.Lock()
	l.subscribers[ch] = struct{}{}
	l.mu.Unlock()

	defer func() {
		l.mu.Lock()
		delete(l.subscribers, ch)
		close(ch)
		l.mu.Unlock()
	}()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case entry := <-ch:
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
