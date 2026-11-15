// Package alerts turns consecutive monitor snapshots into user-facing
// alerts: a distro stopping on its own, a watched dev service dying, and
// memory pressure. It is pure logic; delivery is the caller's job.
package alerts

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"penguinpeek/internal/monitor"
)

// Config selects which alerts are enabled.
type Config struct {
	DistroStopped  bool `json:"distroStopped"`
	ServiceStopped bool `json:"serviceStopped"`
	HighMemory     bool `json:"highMemory"`
}

// NeedsGuestProbe reports whether alerts need guest samples while hidden.
func (c Config) NeedsGuestProbe() bool { return c.ServiceStopped || c.HighMemory }

// Alert is one notification to show.
type Alert struct {
	ID    string
	Title string
	Body  string
}

const (
	memHigh     = 0.90
	memRearm    = 0.80
	memCooldown = 15 * time.Minute
)

// Evaluator remembers the previous state between snapshots.
type Evaluator struct {
	distros     map[string]bool // name -> running, from the last good list
	services    map[string]bool // "distro/id" -> running
	svcDistro   string
	lastSample  time.Time
	memAlerted  bool
	lastMemTime time.Time
}

// Check returns the alerts caused by moving to s.
func (e *Evaluator) Check(s monitor.Snapshot, cfg Config, now time.Time) []Alert {
	var out []Alert
	quiet := func(k string) bool { return slices.Contains(s.Quiet, k) }

	// Distros: skip while an action is in flight or the list failed, so a
	// wsl.exe hiccup never reads as "everything stopped".
	if s.Error == "" && s.Busy == "" && len(s.Distros) > 0 {
		cur := map[string]bool{}
		for _, d := range s.Distros {
			cur[d.Name] = d.Running
		}
		if e.distros != nil && cfg.DistroStopped {
			for name, was := range e.distros {
				isRunning, ok := cur[name]
				if was && ok && !isRunning && !quiet(name) && !strings.HasPrefix(name, "docker-desktop") {
					out = append(out, Alert{
						ID:    "distro-" + name,
						Title: name + " stopped",
						Body:  fmt.Sprintf("The WSL distribution %s stopped without being asked to.", name),
					})
				}
			}
		}
		e.distros = cur
	}

	m := s.Metrics
	if m == nil || !s.Running || !m.SampledAt.After(e.lastSample) {
		if !s.Running {
			e.services = nil // distro alert covers it; start fresh next boot
		}
		return out
	}
	e.lastSample = m.SampledAt

	// Services of the selected distro.
	if s.Selected != e.svcDistro {
		e.services, e.svcDistro = nil, s.Selected
	}
	cur := map[string]bool{}
	for _, sv := range m.Services {
		cur[s.Selected+"/"+sv.ID] = sv.Running
	}
	if e.services != nil && cfg.ServiceStopped {
		for _, sv := range m.Services {
			k := s.Selected + "/" + sv.ID
			if e.services[k] && !sv.Running && !quiet(k) {
				out = append(out, Alert{
					ID:    "service-" + k,
					Title: sv.Name + " stopped",
					Body:  fmt.Sprintf("%s is no longer running in %s.", sv.Name, s.Selected),
				})
			}
		}
	}
	e.services = cur

	// Memory pressure with hysteresis and a cooldown.
	if m.Memory.Total > 0 {
		used := float64(m.Memory.Used) / float64(m.Memory.Total)
		switch {
		case used >= memHigh && !e.memAlerted && now.Sub(e.lastMemTime) >= memCooldown:
			e.memAlerted, e.lastMemTime = true, now
			if cfg.HighMemory {
				out = append(out, Alert{
					ID:    "memory",
					Title: "WSL memory is almost full",
					Body: fmt.Sprintf("%.0f%% of %.1f GB in use. Close something or use Free memory in PenguinPeek.",
						used*100, float64(m.Memory.Total)/(1<<30)),
				})
			}
		case used < memRearm:
			e.memAlerted = false
		}
	}
	return out
}
