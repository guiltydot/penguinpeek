// Command penguinpeek is a lightweight WSL2 monitor that lives in the
// Windows system tray.
package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"penguinpeek/internal/monitor"
	"penguinpeek/internal/settings"
	"penguinpeek/internal/wsl"
)

// version is stamped at build time from the VERSION file (see Taskfile.yml).
var version = "dev"

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/tray/running.png
var trayRunning []byte

//go:embed assets/tray/stopped.png
var trayStopped []byte

//go:embed assets/tray/busy.png
var trayBusy []byte

const (
	eventSnapshot = "snapshot"
	eventSettings = "settings"

	windowWidth   = 360
	defaultHeight = 460
	maxHeight     = 720
	trayOffset    = 8
)

func init() {
	application.RegisterEvent[monitor.Snapshot](eventSnapshot)
	application.RegisterEvent[settings.Settings](eventSettings)
}

func main() {
	cfgDir, err := settings.DefaultDir()
	if err != nil {
		log.Fatal(err)
	}
	store, err := settings.Open(cfgDir, settings.Default(monitor.DefaultWatched))
	if err != nil {
		log.Printf("settings: %v (using defaults)", err)
	}
	cfg := store.Get()

	svc := &PeekService{store: store}
	ns := notifications.New()
	alert := &alerter{ns: ns}

	var flyout *compactWindow
	app := application.New(application.Options{
		Name:        "PenguinPeek",
		Description: "Lightweight WSL2 tray monitor",
		Services: []application.Service{
			application.NewService(svc),
			application.NewService(ns),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.penguinpeek.app",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if flyout != nil {
					flyout.show()
				}
			},
		},
		Windows: application.WindowsOptions{
			// The flyout is destroyed while idle; the tray keeps the app alive.
			DisableQuitOnLastWindowClosed: true,
			// Software rendering is plenty for a 360 px flyout and saves the
			// ~45 MB a hardware GPU process costs. Component updates would
			// download ~60 MB of Edge add-ons (DRM, filter lists, speech) the
			// flyout never uses.
			AdditionalBrowserArgs: []string{"--disable-gpu", "--renderer-process-limit=1", "--disable-component-update"},
			// A cache belongs in Local, not the roaming profile.
			WebviewUserDataPath: webviewDataDir(),
		},
	})
	svc.app = app

	tray := app.SystemTray.New()
	tr := newTrayState(tray, app)

	mon := monitor.New(wsl.NewRunner(), hostAdapter{}, cfg.Distro, cfg.Interval(), cfg.WatchedServices,
		func(s monitor.Snapshot) {
			app.Event.Emit(eventSnapshot, s)
			tr.update(s)
			alert.check(s)
		})
	mon.UseKnownDistros(cfg.Distros, func(ds []wsl.Distro) {
		if _, err := store.Update(func(cur *settings.Settings) { cur.Distros = ds }); err != nil {
			log.Printf("settings: %v", err)
		}
	})
	svc.mon = mon
	tr.mon = mon
	alert.mon = mon
	alert.setConfig(cfg.Alerts)
	svc.onAlerts = alert.setConfig
	svc.testAlert = alert.test

	flyout = newCompactWindow(app, tray, mon, func() bool {
		t := store.Get().Theme
		return t == "dark" || (t == "system" && app.Env.IsDarkMode())
	})
	svc.onResize = flyout.fit

	// The WebView is created on first click, not at startup.
	tray.SetIcon(trayStopped).
		SetMenu(tr.menu).
		WindowOffset(trayOffset).
		OnClick(flyout.toggle)
	tray.SetTooltip("PenguinPeek")
	tr.show = flyout.show
	// Clicking a toast opens the flyout.
	ns.OnNotificationResponse(func(notifications.NotificationResult) { flyout.show() })

	ctx, cancel := context.WithCancel(context.Background())
	app.OnShutdown(cancel)
	go mon.Run(ctx)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// trayState keeps the tray icon, tooltip and menu in sync with snapshots,
// touching the shell only when something visible changed.
type trayState struct {
	tray *application.SystemTray
	app  *application.App
	mon  *monitor.Monitor
	show func()

	menu                                *application.Menu
	start, stop, restart, term, shutAll *application.MenuItem

	mu       sync.Mutex
	selected string
	icon     string
	tooltip  string
	menuKey  string
}

func newTrayState(tray *application.SystemTray, app *application.App) *trayState {
	t := &trayState{tray: tray, app: app, menu: application.NewMenu()}
	t.menu.Add("Open PenguinPeek").OnClick(func(*application.Context) { t.show() })
	t.menu.AddSeparator()
	t.start = t.menu.Add("Start").OnClick(t.action(func(m *monitor.Monitor, d string) error { return m.Start(d) }))
	t.stop = t.menu.Add("Stop").OnClick(t.action(func(m *monitor.Monitor, d string) error { return m.Stop(d) }))
	t.restart = t.menu.Add("Restart").OnClick(t.action(func(m *monitor.Monitor, d string) error { return m.Restart(d) }))
	t.term = t.menu.Add("Open terminal").OnClick(t.action(func(m *monitor.Monitor, d string) error { return m.OpenTerminal(d) }))
	t.menu.AddSeparator()
	t.shutAll = t.menu.Add("Shut down WSL").OnClick(func(*application.Context) {
		go func() {
			if err := t.mon.ShutdownAll(); err != nil {
				log.Printf("shutdown: %v", err)
			}
		}()
	})
	t.menu.AddSeparator()
	t.menu.Add("Quit PenguinPeek").OnClick(func(*application.Context) { app.Quit() })
	return t
}

func (t *trayState) action(f func(*monitor.Monitor, string) error) func(*application.Context) {
	return func(*application.Context) {
		t.mu.Lock()
		d := t.selected
		t.mu.Unlock()
		if d == "" {
			return
		}
		go func() {
			if err := f(t.mon, d); err != nil {
				log.Printf("%s: %v", d, err)
			}
		}()
	}
}

func (t *trayState) update(s monitor.Snapshot) {
	icon, status := "stopped", "Stopped"
	switch {
	case s.Busy != "":
		icon, status = "busy", capitalize(s.Busy)+"…"
	case s.Running:
		icon, status = "running", "Running"
	}
	tooltip := "PenguinPeek"
	if s.Selected != "" {
		tooltip = fmt.Sprintf("PenguinPeek · %s: %s", s.Selected, status)
		if s.Running && s.Metrics != nil && s.Busy == "" {
			m := s.Metrics
			tooltip += fmt.Sprintf("\nCPU %.0f%% · RAM %.1f / %.1f GB", m.CPUPercent,
				float64(m.Memory.Used)/(1<<30), float64(m.Memory.Total)/(1<<30))
		}
	}
	if s.Error != "" {
		tooltip = "PenguinPeek · " + s.Error
	}
	anyRunning := false
	for _, d := range s.Distros {
		anyRunning = anyRunning || d.Running
	}
	menuKey := fmt.Sprintf("%s|%v|%s|%v", s.Selected, s.Running, s.Busy, anyRunning)

	t.mu.Lock()
	t.selected = s.Selected
	setIcon, setTip, setMenu := icon != t.icon, tooltip != t.tooltip, menuKey != t.menuKey
	t.icon, t.tooltip, t.menuKey = icon, tooltip, menuKey
	t.mu.Unlock()

	if setIcon {
		t.tray.SetIcon(map[string][]byte{"running": trayRunning, "stopped": trayStopped, "busy": trayBusy}[icon])
	}
	if setTip {
		t.tray.SetTooltip(tooltip)
	}
	if setMenu {
		idle := s.Busy == "" && s.Selected != ""
		name := s.Selected
		t.start.SetLabel("Start " + name).SetEnabled(idle && !s.Running)
		t.stop.SetLabel("Stop " + name).SetEnabled(idle && s.Running)
		t.restart.SetLabel("Restart " + name).SetEnabled(idle && s.Running)
		t.term.SetEnabled(s.Selected != "")
		t.shutAll.SetEnabled(s.Busy == "" && anyRunning)
		t.menu.Update()
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-'a'+'A') + s[1:]
}
