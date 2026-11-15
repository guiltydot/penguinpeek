package main

import (
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"penguinpeek/internal/monitor"
)

// releaseAfter is how long the hidden flyout keeps its WebView. After that
// the window is destroyed so WebView2's browser, renderer and GPU processes
// (~130-180 MB) can exit; the next tray click recreates it in well under a
// second.
const releaseAfter = 60 * time.Second

// compactWindow owns the lifecycle of the tray flyout.
type compactWindow struct {
	app  *application.App
	tray *application.SystemTray
	mon  *monitor.Monitor
	dark func() bool

	mu        sync.Mutex
	win       *application.WebviewWindow
	height    int
	releasing bool
	timer     *time.Timer
}

func newCompactWindow(app *application.App, tray *application.SystemTray, mon *monitor.Monitor, dark func() bool) *compactWindow {
	return &compactWindow{app: app, tray: tray, mon: mon, dark: dark, height: defaultHeight}
}

// ensure returns the live window, creating it if it was released.
func (c *compactWindow) ensure() *application.WebviewWindow {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.win != nil {
		return c.win
	}
	// Matches the page background, so nothing flashes before it paints.
	bg := application.NewRGB(243, 243, 243)
	if c.dark() {
		bg = application.NewRGB(32, 32, 32)
	}
	w := c.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "compact",
		Title:            "PenguinPeek",
		Width:            windowWidth,
		Height:           c.height,
		Frameless:        true,
		AlwaysOnTop:      true,
		Hidden:           true,
		DisableResize:    true,
		HideOnEscape:     true,
		HideOnFocusLost:  true,
		BackgroundColour: bg,
		BackgroundType:   application.BackgroundTypeSolid,
		URL:              "/",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
		},
	})
	// Alt+F4 hides; only release() really closes the window.
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		c.mu.Lock()
		releasing := c.releasing
		c.mu.Unlock()
		if !releasing {
			w.Hide()
			e.Cancel()
		}
	})
	// Visibility drives the monitor cadence: live metrics only while shown.
	w.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
		c.stopTimer()
		c.mon.SetVisible(true)
	})
	w.OnWindowEvent(events.Common.WindowHide, func(*application.WindowEvent) {
		c.mon.SetVisible(false)
		c.startTimer()
	})
	c.win = w
	c.tray.AttachWindow(w)
	return w
}

// toggle is the tray left-click handler.
func (c *compactWindow) toggle() {
	c.ensure()
	c.tray.ToggleWindow()
}

// show opens the flyout (menu item, second instance launch).
func (c *compactWindow) show() {
	c.ensure()
	c.tray.ShowWindow()
}

// fit resizes the window to its content and keeps it anchored to the tray.
func (c *compactWindow) fit(h int) {
	h = min(max(h, 200), maxHeight)
	c.mu.Lock()
	w := c.win
	changed := h != c.height
	c.height = h
	c.mu.Unlock()
	if w == nil || !changed {
		return
	}
	w.SetSize(windowWidth, h)
	if w.IsVisible() {
		_ = c.tray.PositionWindow(w, trayOffset)
	}
}

func (c *compactWindow) startTimer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(releaseAfter, c.release)
}

func (c *compactWindow) stopTimer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
}

func (c *compactWindow) release() {
	c.mu.Lock()
	w := c.win
	c.mu.Unlock()
	// IsVisible hops to the UI thread, whose hooks take c.mu: never call it
	// while holding the lock.
	if w == nil || w.IsVisible() {
		return
	}
	c.mu.Lock()
	if c.win != w {
		c.mu.Unlock()
		return
	}
	c.win = nil
	c.timer = nil
	c.releasing = true
	c.mu.Unlock()

	w.Close()

	c.mu.Lock()
	c.releasing = false
	c.mu.Unlock()
}
