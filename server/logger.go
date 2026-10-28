package main

// Port of server/logger.ts: newest-first JSON log in <userData>/app.log,
// rotated into app.1.log .. app.4.log past 10MB.

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type LogEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Method    string `json:"method,omitempty"`
	URL       string `json:"url,omitempty"`
	Status    int    `json:"status,omitempty"`
	Data      any    `json:"data,omitempty"`
}

const (
	maxLogs        = 1000
	maxLogFileSize = 10 * 1024 * 1024
	maxLogFiles    = 5
)

type Logger struct {
	mu      sync.Mutex
	logs    []LogEntry
	dir     string
	pending bool
}

var appLogger = newLogger(userDataDir())

func newLogger(dir string) *Logger {
	l := &Logger{dir: dir, logs: []LogEntry{}}
	_ = os.MkdirAll(dir, 0o755)
	if raw, err := os.ReadFile(l.path()); err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &l.logs)
	}
	return l
}

func (l *Logger) path() string { return filepath.Join(l.dir, "app.log") }

func (l *Logger) archived(i int) string {
	return filepath.Join(l.dir, fmt.Sprintf("app.%d.log", i))
}

func (l *Logger) Add(e LogEntry) {
	e.ID = strconv.FormatInt(rand.Int63(), 36)[:7]
	e.Timestamp = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	l.mu.Lock()
	l.logs = append([]LogEntry{e}, l.logs...)
	if len(l.logs) > maxLogs {
		l.logs = l.logs[:maxLogs]
	}
	// Coalesce bursts of entries into a single write.
	if !l.pending {
		l.pending = true
		time.AfterFunc(200*time.Millisecond, func() {
			l.mu.Lock()
			l.pending = false
			l.saveLocked()
			l.mu.Unlock()
		})
	}
	l.mu.Unlock()
}

func (l *Logger) Logs() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]LogEntry(nil), l.logs...)
}

func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = []LogEntry{}
	l.saveLocked()
}

func (l *Logger) saveLocked() {
	raw, err := json.MarshalIndent(l.logs, "", "  ")
	if err != nil || os.WriteFile(l.path(), raw, 0o644) != nil {
		return
	}
	if len(raw) > maxLogFileSize {
		_ = os.Remove(l.archived(maxLogFiles - 1))
		for i := maxLogFiles - 2; i >= 1; i-- {
			_ = os.Rename(l.archived(i), l.archived(i+1))
		}
		_ = os.Rename(l.path(), l.archived(1))
		l.logs = []LogEntry{}
		_ = os.WriteFile(l.path(), []byte("[]"), 0o644)
	}
}
