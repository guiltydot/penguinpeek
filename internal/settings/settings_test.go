package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAndNormalize(t *testing.T) {
	dir := t.TempDir()
	def := Default([]string{"docker"})
	st, err := Open(dir, def)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Get(); got.RefreshSeconds != 60 || got.Theme != "system" {
		t.Fatalf("defaults %+v", got)
	}
	if _, err := st.Update(func(s *Settings) {
		s.Distro = "Ubuntu"
		s.Theme = "neon"      // invalid -> system
		s.RefreshSeconds = 99 // invalid -> 60
	}); err != nil {
		t.Fatal(err)
	}
	st2, err := Open(dir, def)
	if err != nil {
		t.Fatal(err)
	}
	got := st2.Get()
	if got.Distro != "Ubuntu" || got.Theme != "system" || got.RefreshSeconds != 60 || len(got.WatchedServices) != 1 {
		t.Fatalf("reloaded %+v", got)
	}
}

func TestOldFileGetsDefaultAlerts(t *testing.T) {
	dir := t.TempDir()
	old := `{"distro":"","theme":"system","refreshSeconds":2,"watchedServices":[]}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Open(dir, Default(nil))
	if err != nil {
		t.Fatal(err)
	}
	got := st.Get()
	if got.Alerts == nil || !got.Alerts.DistroStopped || !got.Alerts.HighMemory {
		t.Fatalf("alerts %+v", got.Alerts)
	}
	if len(got.WatchedServices) != 0 {
		t.Fatalf("an explicit empty list must be kept: %v", got.WatchedServices)
	}
}

func TestCorruptFileFallsBack(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Open(dir, Default(nil))
	if err == nil {
		t.Fatal("want parse error")
	}
	if st.Get().RefreshSeconds != 60 {
		t.Fatal("want defaults")
	}
}
