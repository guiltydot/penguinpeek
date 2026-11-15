package monitor

import (
	"context"
	"os"
	"testing"
	"time"

	"penguinpeek/internal/wsl"
)

// TestLiveSnapshot runs one visible tick against real WSL when PEEK_LIVE=1.
func TestLiveSnapshot(t *testing.T) {
	if os.Getenv("PEEK_LIVE") == "" {
		t.Skip("set PEEK_LIVE=1 to probe the real wsl.exe")
	}
	m := New(wsl.NewRunner(), nil, "", 2*time.Second, DefaultWatched, nil)
	m.SetVisible(true)
	m.tick(context.Background())
	s := m.Snapshot()
	t.Logf("selected=%s running=%v err=%q", s.Selected, s.Running, s.Error)
	if s.Metrics != nil {
		t.Logf("cpu=%.1f services=%+v", s.Metrics.CPUPercent, s.Metrics.Services)
	}
}

// TestLiveContainers lists real containers and, when PEEK_LIVE_PROJECT names
// a throwaway compose project, stops and starts it.
func TestLiveContainers(t *testing.T) {
	if os.Getenv("PEEK_LIVE") == "" {
		t.Skip("set PEEK_LIVE=1 to probe the real wsl.exe")
	}
	m := New(wsl.NewRunner(), nil, "", 2*time.Second, DefaultWatched, nil)
	m.SetVisible(true)
	m.SetContainersWanted(true)
	m.tick(context.Background())
	s := m.Snapshot()
	if s.Metrics == nil {
		t.Fatalf("no metrics: %+v", s)
	}
	t.Logf("containersError=%q", s.Metrics.ContainersError)
	for _, c := range s.Metrics.Containers {
		t.Logf("%s %-18s project=%-8q state=%-8s %s", c.ID[:12], c.Name, c.Project, c.State, c.Status)
	}
	project := os.Getenv("PEEK_LIVE_PROJECT")
	if project == "" {
		return
	}
	count := func(want bool) int {
		n := 0
		for _, c := range m.Snapshot().Metrics.Containers {
			if c.Project == project && c.Running() == want {
				n++
			}
		}
		return n
	}
	for _, action := range []string{"stop", "start", "restart"} {
		if err := m.ContainerAction(s.Selected, action, "project:"+project); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		m.tick(context.Background())
		t.Logf("after %s: running=%d stopped=%d", action, count(true), count(false))
		if (action == "stop") != (count(true) == 0) {
			t.Errorf("%s did not take effect", action)
		}
	}
}
