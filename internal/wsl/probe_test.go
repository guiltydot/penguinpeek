package wsl

import (
	"context"
	"errors"
	"os"
	"testing"
)

// TestProbeLive runs against the real WSL install when PEEK_LIVE=1.
func TestProbeLive(t *testing.T) {
	if os.Getenv("PEEK_LIVE") == "" {
		t.Skip("set PEEK_LIVE=1 to probe the real wsl.exe")
	}
	r := NewRunner()
	ds, err := r.List(context.Background())
	t.Logf("distros=%+v err=%v", ds, err)
	for _, d := range ds {
		if !d.Running {
			continue
		}
		for _, lv := range []Level{LevelLite, LevelLive, LevelFull} {
			s, err := r.Metrics(context.Background(), d.Name, lv == LevelFull, lv)
			if err != nil {
				t.Errorf("%s level %d: %v", d.Name, lv, err)
				continue
			}
			t.Logf("%s level %d: up=%.0f cpu=%v mem=%+v procs=%d in-container=%v", d.Name, lv, s.UptimeSec, s.CPU, s.Mem, len(s.Procs), s.ContainerProcs)
			t.Logf("disk=%+v home=%q os=%s/%s systemd=%v ports=%+v", s.Disk, s.Home, s.OSID, s.OSLike, s.Systemd, s.Ports)
			if lv == LevelFull && (s.Disk.Total == 0 || s.Home == "" || s.OSID == "") {
				t.Errorf("%s: full sample missing disk/home/os", d.Name)
			}
			if lv == LevelLite && (len(s.Ports) > 0 || s.Disk.Total != 0) {
				t.Errorf("%s: lite sample collected extra sections", d.Name)
			}
		}
	}
}

// TestRootActionsLive checks the root exec path with harmless actions only:
// a service name that cannot exist, and dropping the page cache.
func TestRootActionsLive(t *testing.T) {
	if os.Getenv("PEEK_LIVE") == "" {
		t.Skip("set PEEK_LIVE=1 to probe the real wsl.exe")
	}
	r := NewRunner()
	ds, _ := r.List(context.Background())
	for _, d := range ds {
		if !d.Running {
			continue
		}
		err := r.ServiceControl(context.Background(), d.Name, "start", []string{"penguinpeek-does-not-exist"})
		if !errors.Is(err, ErrNotInstalled) {
			t.Errorf("%s: want ErrNotInstalled, got %v", d.Name, err)
		}
		if err := r.DropCaches(context.Background(), d.Name); err != nil {
			t.Errorf("%s: drop caches: %v", d.Name, err)
		}
		return
	}
	t.Skip("no running distro")
}
