//go:build smoke

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Run with: go test -tags smoke -run Smoke
// Uses the token from the Electron build's config.json against live Storytel.
func TestSmokeBookshelfWithMigratedToken(t *testing.T) {
	base, _ := os.UserConfigDir()
	raw, err := os.ReadFile(filepath.Join(base, "storytel-player", "config.json"))
	if err != nil {
		t.Skip("no Electron config.json")
	}
	var cfg struct{ Token string }
	_ = json.Unmarshal(raw, &cfg)
	mux := NewAPI(t.TempDir()).Routes()

	for _, path := range []string{"/api/auth/status", "/api/bookshelf", "/api/bookmark-positional"} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var body any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		summary := ""
		switch b := body.(type) {
		case map[string]any:
			if books, ok := b["books"].([]any); ok {
				summary = "books=" + itoa(len(books))
			} else {
				raw, _ := json.Marshal(b)
				summary = string(raw)
			}
		case []any:
			summary = "items=" + itoa(len(b))
		}
		t.Logf("%s -> %d %s", path, rec.Code, summary)
		if rec.Code != http.StatusOK {
			t.Errorf("%s returned %d", path, rec.Code)
		}
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }
