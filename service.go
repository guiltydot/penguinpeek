package main

import (
	"fmt"
	"slices"

	"github.com/wailsapp/wails/v3/pkg/application"

	"penguinpeek/internal/host"
	"penguinpeek/internal/monitor"
	"penguinpeek/internal/settings"
	"penguinpeek/internal/wsl"
)

// ServiceInfo describes a detectable dev service for the settings UI.
type ServiceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PeekService is the IPC surface for the frontend. The frontend never polls
// it for metrics: it calls Snapshot once on load and then listens for the
// "snapshot" event.
type PeekService struct {
	mon      *monitor.Monitor
	store    *settings.Store
	app      *application.App
	onResize func(height int)
	// onAlerts applies alert preferences (guest probing while hidden).
	onAlerts  func(*settings.Alerts)
	testAlert func() error
}

// Version returns the app's semantic version.
func (s *PeekService) Version() string { return version }

// Snapshot returns the latest cached snapshot without touching WSL.
func (s *PeekService) Snapshot() monitor.Snapshot {
	return s.mon.Snapshot()
}

// Settings returns the current preferences.
func (s *PeekService) Settings() settings.Settings {
	return s.store.Get()
}

// SaveSettings persists preferences and applies them.
func (s *PeekService) SaveSettings(next settings.Settings) (settings.Settings, error) {
	saved, err := s.store.Update(func(cur *settings.Settings) {
		cur.Theme = next.Theme
		cur.RefreshSeconds = next.RefreshSeconds
		cur.WatchedServices = next.WatchedServices
		if next.Alerts != nil {
			cur.Alerts = next.Alerts
		}
	})
	if err != nil {
		return saved, err
	}
	s.mon.SetInterval(saved.Interval())
	s.mon.SetWatched(saved.WatchedServices)
	if s.onAlerts != nil {
		s.onAlerts(saved.Alerts)
	}
	s.app.Event.Emit(eventSettings, saved)
	return saved, nil
}

// KnownServices lists the services PenguinPeek can detect.
func (s *PeekService) KnownServices() []ServiceInfo {
	out := make([]ServiceInfo, 0, len(monitor.Catalog))
	for _, d := range monitor.Catalog {
		out = append(out, ServiceInfo{ID: d.ID, Name: d.Name})
	}
	return out
}

// Select focuses the compact window on another distro and remembers it.
func (s *PeekService) Select(name string) error {
	if !s.known(name) {
		return fmt.Errorf("unknown distribution %q", name)
	}
	s.mon.Select(name)
	_, err := s.store.Update(func(cur *settings.Settings) { cur.Distro = name })
	return err
}

// Start boots a distro.
func (s *PeekService) Start(name string) error { return s.guard(name, s.mon.Start) }

// Stop terminates a distro.
func (s *PeekService) Stop(name string) error { return s.guard(name, s.mon.Stop) }

// Restart terminates and boots a distro.
func (s *PeekService) Restart(name string) error { return s.guard(name, s.mon.Restart) }

// OpenTerminal opens a shell in the distro.
func (s *PeekService) OpenTerminal(name string) error { return s.guard(name, s.mon.OpenTerminal) }

// Refresh asks for an immediate update (e.g. a manual refresh button).
func (s *PeekService) Refresh() { s.mon.Refresh() }

// RefreshDistros re-reads the list of installed distributions.
func (s *PeekService) RefreshDistros() { s.mon.RefreshDistros() }

// FitHeight resizes the compact window to its content and keeps it anchored
// to the tray.
func (s *PeekService) FitHeight(height int) {
	if s.onResize != nil && height > 0 {
		s.onResize(height)
	}
}

func (s *PeekService) guard(name string, f func(string) error) error {
	if !s.known(name) {
		return fmt.Errorf("unknown distribution %q", name)
	}
	return f(name)
}

// known rejects names that are not registered distros, so the frontend can
// never pass arbitrary arguments through to wsl.exe.
func (s *PeekService) known(name string) bool {
	return slices.ContainsFunc(s.mon.Snapshot().Distros, func(d wsl.Distro) bool { return d.Name == name })
}

// Autostart reports whether PenguinPeek starts when you sign in to Windows.
func (s *PeekService) Autostart() (bool, error) { return host.AutostartEnabled() }

// SetAutostart turns launch-at-login on or off for this exe.
func (s *PeekService) SetAutostart(on bool) error { return host.SetAutostart(on) }

// ServiceControl starts or stops a known dev service ("start" or "stop").
func (s *PeekService) ServiceControl(distro, id, action string) error {
	if action != "start" && action != "stop" {
		return fmt.Errorf("unsupported action %q", action)
	}
	return s.guard(distro, func(d string) error { return s.mon.ServiceControl(d, id, action) })
}

// SetContainersWatch tells the monitor whether the Containers tab is open,
// so containers are only listed while someone looks at them.
func (s *PeekService) SetContainersWatch(on bool) { s.mon.SetContainersWanted(on) }

// ContainerAction starts, stops or restarts a Docker container (by ID) or a
// compose project ("project:<name>") in the distro.
func (s *PeekService) ContainerAction(distro, action, target string) error {
	switch action {
	case "start", "stop", "restart":
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
	return s.guard(distro, func(d string) error { return s.mon.ContainerAction(d, action, target) })
}

// OpenPort opens http://localhost:<port> in the default browser. WSL2
// forwards localhost, so guest servers are reachable from Windows.
func (s *PeekService) OpenPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	return s.app.Browser.OpenURL(fmt.Sprintf("http://localhost:%d/", port))
}

// FreeMemory drops Linux's page cache so Windows can reclaim the RAM.
func (s *PeekService) FreeMemory(distro string) error { return s.guard(distro, s.mon.FreeMemory) }

// TrimDisk discards unused blocks in the distro's filesystem.
func (s *PeekService) TrimDisk(distro string) (string, error) {
	if !s.known(distro) {
		return "", fmt.Errorf("unknown distribution %q", distro)
	}
	return s.mon.Trim(distro)
}

// CompactDisk shuts down WSL and compacts the distro's disk file in an
// elevated window (Windows asks for administrator approval).
func (s *PeekService) CompactDisk(distro string) error { return s.guard(distro, s.mon.CompactDisk) }

// OpenFiles opens the distro's home folder in File Explorer.
func (s *PeekService) OpenFiles(distro string) error { return s.guard(distro, s.mon.OpenFiles) }

// OpenVSCode opens VS Code connected to the distro.
func (s *PeekService) OpenVSCode(distro string) error { return s.guard(distro, s.mon.OpenVSCode) }

// TestNotification shows a sample alert.
func (s *PeekService) TestNotification() error {
	if s.testAlert == nil {
		return fmt.Errorf("notifications are not available")
	}
	return s.testAlert()
}
