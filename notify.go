package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"penguinpeek/internal/alerts"
	"penguinpeek/internal/host"
	"penguinpeek/internal/monitor"
	"penguinpeek/internal/settings"
)

// alerter feeds snapshots to the alert evaluator and shows Windows toasts.
// Snapshots arrive from the monitor loop and from action goroutines, so it
// serialises access itself.
type alerter struct {
	ns  *notifications.NotificationService
	mon *monitor.Monitor

	mu  sync.Mutex
	ev  alerts.Evaluator
	cfg alerts.Config
}

func (a *alerter) setConfig(s *settings.Alerts) {
	if s == nil {
		s = settings.DefaultAlerts()
	}
	cfg := alerts.Config{DistroStopped: s.DistroStopped, ServiceStopped: s.ServiceStopped, HighMemory: s.HighMemory}
	a.mu.Lock()
	a.cfg = cfg
	a.mu.Unlock()
	if a.mon != nil {
		a.mon.SetProbeHidden(cfg.NeedsGuestProbe())
	}
}

func (a *alerter) check(s monitor.Snapshot) {
	a.mu.Lock()
	out := a.ev.Check(s, a.cfg, time.Now())
	a.mu.Unlock()
	for _, al := range out {
		go func() {
			err := a.ns.SendNotification(notifications.NotificationOptions{
				ID:    fmt.Sprintf("%s-%d", al.ID, time.Now().UnixNano()),
				Title: al.Title,
				Body:  al.Body,
			})
			if err != nil {
				log.Printf("notification: %v", err)
			}
		}()
	}
}

// test sends a sample notification so you can check Windows shows them
// (Focus / Do Not Disturb can hide toasts).
func (a *alerter) test() error {
	return a.ns.SendNotification(notifications.NotificationOptions{
		ID:    fmt.Sprintf("test-%d", time.Now().UnixNano()),
		Title: "PenguinPeek notifications work",
		Body:  "You'll see alerts like this when a distro or service stops, or memory runs high.",
	})
}

// hostAdapter exposes the host package to the monitor's Host interface.
type hostAdapter struct{}

func (hostAdapter) VMMemory() (uint64, error)              { return host.VMMemory() }
func (hostAdapter) VHDXPath(distro string) (string, error) { return host.VHDXPath(distro) }
func (hostAdapter) FileSize(path string) (uint64, error)   { return host.FileSize(path) }
func (hostAdapter) CompactVHDX(path string) error          { return host.CompactVHDX(path) }
