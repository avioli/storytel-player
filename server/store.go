package main

// JSON key/value store, compatible with the electron-store config.json the
// Electron build wrote: keys are dot-paths ("window.bounds").

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Store struct {
	mu   sync.Mutex
	path string
	data map[string]any
}

// userDataDir is this app's own data directory. It is deliberately separate
// from the Electron build's, so both can be installed side by side.
func userDataDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "storytel-player-wails")
}

func NewStore() *Store {
	dir := userDataDir()
	_ = os.MkdirAll(dir, 0o755)
	s := &Store{path: filepath.Join(dir, "config.json"), data: map[string]any{}}

	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		// First run: carry over settings and the login from the Electron build.
		if base, e := os.UserConfigDir(); e == nil {
			raw, err = os.ReadFile(filepath.Join(base, "storytel-player", "config.json"))
		}
	}
	if err == nil {
		_ = json.Unmarshal(raw, &s.data)
		if s.data == nil {
			s.data = map[string]any{}
		}
		s.saveLocked()
	}
	return s
}

func (s *Store) Get(key string) any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var cur any = s.data
	for _, part := range strings.Split(key, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[part]
	}
	return cur
}

func (s *Store) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	parts := strings.Split(key, ".")
	m := s.data
	for _, part := range parts[:len(parts)-1] {
		next, ok := m[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[part] = next
		}
		m = next
	}
	m[parts[len(parts)-1]] = value
	s.saveLocked()
}

func (s *Store) Remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	parts := strings.Split(key, ".")
	m := s.data
	for _, part := range parts[:len(parts)-1] {
		next, ok := m[part].(map[string]any)
		if !ok {
			return
		}
		m = next
	}
	delete(m, parts[len(parts)-1])
	s.saveLocked()
}

func (s *Store) saveLocked() {
	raw, err := json.MarshalIndent(s.data, "", "\t")
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, raw, 0o600) == nil {
		_ = os.Rename(tmp, s.path)
	}
}
