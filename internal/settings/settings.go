// Package settings persists user preferences as JSON under the user config
// directory (%APPDATA%\PenguinPeek on Windows).
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"penguinpeek/internal/wsl"
)

// Settings are the user preferences. Keep fields JSON-friendly; the struct
// is exchanged with the frontend as-is.
type Settings struct {
	// Distro selected in the compact window; empty means the WSL default.
	Distro string `json:"distro"`
	// Theme is "system", "light" or "dark".
	Theme string `json:"theme"`
	// RefreshSeconds is the metrics interval while the window is visible.
	RefreshSeconds int `json:"refreshSeconds"`
	// WatchedServices are always listed, running or not.
	WatchedServices []string `json:"watchedServices"`
	// Alerts is nil in files written before alerts existed; Normalize
	// fills in the defaults (all on).
	Alerts *Alerts `json:"alerts"`
	// Distros caches the registered WSL2 distros so startup needs no
	// `wsl --list`; refreshed from the distro picker or when a new one runs.
	Distros []wsl.Distro `json:"distros,omitempty"`
}

// Alerts selects which Windows notifications are shown.
type Alerts struct {
	DistroStopped  bool `json:"distroStopped"`
	ServiceStopped bool `json:"serviceStopped"`
	HighMemory     bool `json:"highMemory"`
}

// DefaultAlerts enables every alert.
func DefaultAlerts() *Alerts {
	return &Alerts{DistroStopped: true, ServiceStopped: true, HighMemory: true}
}

// AllowedRefresh are the intervals offered in the UI.
var AllowedRefresh = []int{2, 5, 10, 60}

// Default returns the defaults applied for a missing or invalid file.
func Default(watched []string) Settings {
	return Settings{Theme: "system", RefreshSeconds: 60, WatchedServices: watched, Alerts: DefaultAlerts()}
}

// Interval returns RefreshSeconds as a duration.
func (s Settings) Interval() time.Duration {
	return time.Duration(s.RefreshSeconds) * time.Second
}

// Normalize fixes out-of-range values.
func (s Settings) Normalize(def Settings) Settings {
	if s.Theme != "light" && s.Theme != "dark" {
		s.Theme = "system"
	}
	if !slices.Contains(AllowedRefresh, s.RefreshSeconds) {
		s.RefreshSeconds = def.RefreshSeconds
	}
	if s.WatchedServices == nil {
		s.WatchedServices = def.WatchedServices
	}
	if s.Alerts == nil {
		s.Alerts = DefaultAlerts()
	}
	return s
}

// Store loads and saves settings.
type Store struct {
	path string
	def  Settings
	mu   sync.Mutex
	cur  Settings
}

// Open loads settings from dir/settings.json, falling back to defaults.
func Open(dir string, def Settings) (*Store, error) {
	st := &Store{path: filepath.Join(dir, "settings.json"), def: def, cur: def.Normalize(def)}
	b, err := os.ReadFile(st.path)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("read settings %q: %w", st.path, err)
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return st, fmt.Errorf("parse settings %q: %w", st.path, err)
	}
	st.cur = s.Normalize(def)
	return st, nil
}

// DefaultDir is the per-user config directory for PenguinPeek.
func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PenguinPeek"), nil
}

// Get returns the current settings.
func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.cur
}

// Update applies f, normalizes, and writes the file atomically.
func (st *Store) Update(f func(*Settings)) (Settings, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	next := st.cur
	next.WatchedServices = slices.Clone(st.cur.WatchedServices)
	next.Distros = slices.Clone(st.cur.Distros)
	if st.cur.Alerts != nil {
		a := *st.cur.Alerts
		next.Alerts = &a
	}
	f(&next)
	next = next.Normalize(st.def)
	if err := os.MkdirAll(filepath.Dir(st.path), 0o755); err != nil {
		return st.cur, fmt.Errorf("create settings dir: %w", err)
	}
	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return st.cur, err
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return st.cur, fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp, st.path); err != nil {
		return st.cur, fmt.Errorf("save settings: %w", err)
	}
	st.cur = next
	return next, nil
}
