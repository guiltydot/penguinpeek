package monitor

import (
	"context"
	"os/exec"
	"slices"
	"sync"
	"testing"
	"time"

	"penguinpeek/internal/wsl"
)

type fakeBackend struct {
	mu          sync.Mutex
	distros     []wsl.Distro
	listCalls   int
	runCalls    int
	metricCalls []string
	levels      []wsl.Level
	dockerd     bool
	containers  []wsl.Container
	listedCtrs  int
	actions     [][]string // action followed by ids
	ctrProcs    map[string]string
}

func (f *fakeBackend) List(context.Context) ([]wsl.Distro, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	return append([]wsl.Distro(nil), f.distros...), nil
}

func (f *fakeBackend) Running(context.Context) (map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runCalls++
	r := map[string]bool{}
	for _, d := range f.distros {
		r[d.Name] = d.Running
	}
	return r, nil
}

func (f *fakeBackend) Metrics(_ context.Context, d string, two bool, lv wsl.Level) (wsl.Sample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metricCalls = append(f.metricCalls, d)
	f.levels = append(f.levels, lv)
	s := wsl.Sample{
		UptimeSec: 60,
		CPU:       []wsl.CPUSample{{Idle: 50, Total: 100}},
		Mem:       wsl.Memory{Total: 8 << 30, Used: 2 << 30},
		Procs:     map[string]bool{"redis-server": true, "dockerd": f.dockerd},

		ContainerProcs: f.ctrProcs,
	}
	if two {
		s.CPU = append(s.CPU, wsl.CPUSample{Idle: 75, Total: 200})
	}
	if lv >= wsl.LevelLive {
		s.Ports = []wsl.Port{{Port: 5173, Process: "node"}}
	}
	if lv == wsl.LevelFull {
		s.Disk = wsl.Disk{Used: 10 << 30, Total: 100 << 30}
		s.Home, s.OSID = "/home/u", "ubuntu"
	}
	return s, nil
}

func (f *fakeBackend) KeepAlive(string) (*exec.Cmd, error)     { return nil, nil }
func (f *fakeBackend) Terminate(context.Context, string) error { return nil }
func (f *fakeBackend) Shutdown(context.Context) error          { return nil }
func (f *fakeBackend) OpenTerminal(string) error               { return nil }
func (f *fakeBackend) ServiceControl(context.Context, string, string, []string) error {
	return nil
}
func (f *fakeBackend) DropCaches(context.Context, string) error { return nil }
func (f *fakeBackend) Trim(context.Context, string) (string, error) {
	return "/: 0 B trimmed", nil
}
func (f *fakeBackend) OpenFiles(string, string) error  { return nil }
func (f *fakeBackend) OpenVSCode(string, string) error { return nil }
func (f *fakeBackend) Containers(context.Context, string) ([]wsl.Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listedCtrs++
	return slices.Clone(f.containers), nil
}
func (f *fakeBackend) ContainerAction(_ context.Context, _ string, action string, ids []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actions = append(f.actions, append([]string{action}, ids...))
	return nil
}

func newFake() *fakeBackend {
	return &fakeBackend{distros: []wsl.Distro{
		{Name: "Ubuntu", Default: true, Version: 2, Running: true},
		{Name: "Debian", Version: 2, Running: false},
	}}
}

func TestHiddenNeverExecsIntoGuest(t *testing.T) {
	be := newFake()
	m := New(be, nil, "", 2*time.Second, DefaultWatched, nil)
	m.tick(context.Background()) // first tick: full list
	m.tick(context.Background()) // later hidden tick: running list only
	if len(be.metricCalls) != 0 {
		t.Fatalf("metrics called while hidden: %v", be.metricCalls)
	}
	// The fake List does not call Running (the real runner does), so the
	// second tick is the only Running call.
	if be.listCalls != 1 || be.runCalls != 1 {
		t.Fatalf("list=%d running=%d", be.listCalls, be.runCalls)
	}
	if s := m.Snapshot(); s.Selected != "Ubuntu" || !s.Running {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestVisibleStoppedDistroIsNotBooted(t *testing.T) {
	be := newFake()
	m := New(be, nil, "Debian", 2*time.Second, DefaultWatched, nil)
	m.visible = true
	m.tick(context.Background())
	if len(be.metricCalls) != 0 {
		t.Fatalf("exec into stopped distro: %v", be.metricCalls)
	}
	if s := m.Snapshot(); s.Running || s.Metrics != nil {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestVisibleRunningProducesMetrics(t *testing.T) {
	be := newFake()
	var got []Snapshot
	m := New(be, nil, "", 2*time.Second, []string{"docker"}, func(s Snapshot) { got = append(got, s) })
	m.visible = true
	m.tick(context.Background())
	s := m.Snapshot()
	if s.Metrics == nil || !s.Live {
		t.Fatalf("no metrics: %+v", s)
	}
	// Two-sample path: (100-25)/100 busy = 75%.
	if s.Metrics.CPUPercent != 75 {
		t.Errorf("cpu %v", s.Metrics.CPUPercent)
	}
	// docker is watched (shown stopped); redis detected running.
	if len(s.Metrics.Services) != 2 || s.Metrics.Services[0].ID != "docker" || s.Metrics.Services[0].Running ||
		s.Metrics.Services[1].ID != "redis" || !s.Metrics.Services[1].Running {
		t.Errorf("services %+v", s.Metrics.Services)
	}
	if len(got) != 1 {
		t.Errorf("updates %d", len(got))
	}
}

func TestUnknownSelectionFallsBackToDefault(t *testing.T) {
	be := newFake()
	m := New(be, nil, "Gone", time.Second, nil, nil)
	m.tick(context.Background())
	if s := m.Snapshot(); s.Selected != "Ubuntu" {
		t.Fatalf("selected %q", s.Selected)
	}
}

func TestWSL1DistrosAreHidden(t *testing.T) {
	be := newFake()
	be.distros = append(be.distros, wsl.Distro{Name: "Legacy", Version: 1, Running: true})
	m := New(be, nil, "", time.Second, nil, nil)
	m.tick(context.Background())
	for _, d := range m.Snapshot().Distros {
		if d.Name == "Legacy" {
			t.Fatal("WSL1 distro listed")
		}
	}
}

func TestDistroListIsCachedAndPersisted(t *testing.T) {
	be := newFake()
	var saved [][]wsl.Distro
	m := New(be, nil, "", time.Second, nil, nil)
	m.UseKnownDistros(nil, func(ds []wsl.Distro) { saved = append(saved, ds) })
	m.visible = true
	for range 3 {
		m.tick(context.Background())
	}
	if be.listCalls != 1 || len(saved) != 1 || saved[0][0].Running {
		t.Fatalf("list=%d saved=%v", be.listCalls, saved)
	}
	// A distro installed meanwhile shows up once it runs.
	be.mu.Lock()
	be.distros = append(be.distros, wsl.Distro{Name: "Fedora", Version: 2, Running: true})
	be.mu.Unlock()
	m.tick(context.Background())
	if be.listCalls != 2 || len(saved) != 2 || len(m.Snapshot().Distros) != 3 {
		t.Fatalf("new distro not picked up: list=%d saved=%d", be.listCalls, len(saved))
	}
	m.RefreshDistros()
	m.tick(context.Background())
	if be.listCalls != 3 || len(saved) != 2 {
		t.Fatalf("manual refresh: list=%d saved=%d (unchanged list must not be saved)", be.listCalls, len(saved))
	}
}

func TestCachedDistrosSkipStartupList(t *testing.T) {
	be := newFake()
	m := New(be, nil, "", time.Second, nil, nil)
	m.UseKnownDistros(registered(be.distros), nil)
	m.tick(context.Background())
	if be.listCalls != 0 || be.runCalls != 1 || !m.Snapshot().Running {
		t.Fatalf("list=%d running=%d snap=%+v", be.listCalls, be.runCalls, m.Snapshot())
	}
}

func TestSampleLevels(t *testing.T) {
	be := newFake()
	m := New(be, nil, "", time.Second, nil, nil)
	m.visible = true
	m.tick(context.Background())
	m.tick(context.Background())
	m.visible = false
	m.probeHidden = true
	m.tick(context.Background())
	want := []wsl.Level{wsl.LevelFull, wsl.LevelLive, wsl.LevelLite}
	if !slices.Equal(be.levels, want) {
		t.Fatalf("levels %v, want %v", be.levels, want)
	}
	// Full-sample values and ports are carried into lighter samples.
	mt := m.Snapshot().Metrics
	if mt.Disk.Total != 100<<30 || mt.Home != "/home/u" || mt.OSID != "ubuntu" || len(mt.Ports) != 1 {
		t.Fatalf("carry-forward lost: %+v", mt)
	}
}

func TestContainersListedOnlyWhenWanted(t *testing.T) {
	be := newFake()
	be.containers = []wsl.Container{{ID: "aaaaaaaaaaaa", Name: "web", State: "running"}}
	m := New(be, nil, "", time.Second, nil, nil)
	m.SetContainersWanted(true)
	m.tick(context.Background()) // hidden
	m.visible = true
	be.dockerd = false
	m.tick(context.Background()) // docker not running
	if be.listedCtrs != 0 {
		t.Fatalf("listed %d times while hidden or docker stopped", be.listedCtrs)
	}
	if m.Snapshot().Metrics.Containers != nil {
		t.Fatal("containers set without listing")
	}
	be.dockerd = true
	m.tick(context.Background())
	if be.listedCtrs != 1 || len(m.Snapshot().Metrics.Containers) != 1 {
		t.Fatalf("listed=%d snapshot=%+v", be.listedCtrs, m.Snapshot().Metrics)
	}
	m.SetContainersWanted(false)
	m.tick(context.Background())
	if be.listedCtrs != 1 || m.Snapshot().Metrics.Containers != nil {
		t.Fatalf("still listing after tab closed: %d", be.listedCtrs)
	}
}

const bentoID = "abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abc1"

func TestServiceInContainerIsNamedNotControllable(t *testing.T) {
	be := newFake()
	be.dockerd = true
	be.ctrProcs = map[string]string{"nginx": bentoID}
	be.containers = []wsl.Container{{ID: bentoID, Name: "bentopdf", State: "running"}}
	m := New(be, nil, "", time.Second, nil, nil)
	m.visible = true
	m.tick(context.Background())
	nginx := func() ServiceStatus {
		for _, s := range m.Snapshot().Metrics.Services {
			if s.ID == "nginx" {
				return s
			}
		}
		t.Fatal("nginx not listed")
		return ServiceStatus{}
	}
	got := nginx()
	if !got.Running || got.Controllable || !got.InContainer || got.Container != "bentopdf" {
		t.Fatalf("nginx %+v", got)
	}
	if m.Snapshot().Metrics.Containers != nil {
		t.Fatal("a name lookup must not fill the Containers tab")
	}
	// The name is cached: no further listings while the tab is closed.
	m.tick(context.Background())
	if be.listedCtrs != 1 || nginx().Container != "bentopdf" {
		t.Fatalf("listed=%d nginx=%+v", be.listedCtrs, nginx())
	}
}

func TestDetectServicesPrefersDistroProcess(t *testing.T) {
	ss := detectServices(guestState{procs: map[string]bool{"postgres": true}, inContainer: map[string]string{"postgres": bentoID, "nginx": ""}},
		nil, nil)
	if len(ss) != 2 {
		t.Fatalf("services %+v", ss)
	}
	if pg := ss[0]; pg.ID != "postgres" || !pg.Controllable || pg.InContainer {
		t.Errorf("distro postgres %+v", pg)
	}
	if ng := ss[1]; ng.ID != "nginx" || ng.Controllable || !ng.InContainer || ng.Container != "" {
		t.Errorf("container nginx %+v", ng)
	}
}

func TestDetectServicesTailscaleAndSSH(t *testing.T) {
	byID := func(ss []ServiceStatus) map[string]ServiceStatus {
		out := map[string]ServiceStatus{}
		for _, s := range ss {
			out[s.ID] = s
		}
		return out
	}
	all := []string{"tailscale", "tailscale-ssh", "ssh"}

	got := byID(detectServices(guestState{procs: map[string]bool{"tailscaled": true}, tailscaleSSH: true}, nil, nil))
	if ts := got["tailscale"]; !ts.Running || !ts.Controllable {
		t.Errorf("tailscale %+v", ts)
	}
	if ss := got["tailscale-ssh"]; !ss.Running || ss.Controllable {
		t.Errorf("tailscale ssh %+v", ss)
	}
	if _, ok := got["ssh"]; ok {
		t.Error("unwatched stopped OpenSSH listed")
	}

	// Tailscale SSH is down when tailscaled is, even if the pref is on.
	got = byID(detectServices(guestState{procs: map[string]bool{}, tailscaleSSH: true}, nil, all))
	if got["tailscale"].Running || got["tailscale-ssh"].Running || got["ssh"].Running {
		t.Errorf("stopped %+v", got)
	}

	// Socket-activated sshd: root-owned listener on :22 and no sshd process.
	got = byID(detectServices(guestState{procs: map[string]bool{}, ports: []wsl.Port{{Port: 22}}}, nil, nil))
	if s := got["ssh"]; !s.Running || s.Detail != "socket-activated" {
		t.Errorf("socket ssh %+v", s)
	}
	got = byID(detectServices(guestState{procs: map[string]bool{}, ports: []wsl.Port{{Port: 22, Loopback: true}}}, nil, nil))
	if _, ok := got["ssh"]; ok {
		t.Error("loopback :22 taken for sshd")
	}
	got = byID(detectServices(guestState{procs: map[string]bool{"sshd-session": true}}, nil, nil))
	if s := got["ssh"]; !s.Running || s.Detail != "" {
		t.Errorf("sshd-session %+v", s)
	}
}

func TestContainerActionTargets(t *testing.T) {
	be := newFake()
	be.dockerd = true
	be.containers = []wsl.Container{
		{ID: "aaaaaaaaaaaa", Name: "web", State: "running", Project: "shop"},
		{ID: "bbbbbbbbbbbb", Name: "db", State: "exited", Project: "shop"},
		{ID: "cccccccccccc", Name: "solo", State: "exited"},
	}
	m := New(be, nil, "", time.Second, nil, nil)
	m.visible = true
	m.SetContainersWanted(true)
	m.tick(context.Background())
	for _, tc := range []struct {
		action, target string
		want           []string
	}{
		{"stop", "project:shop", []string{"stop", "aaaaaaaaaaaa"}},
		{"start", "project:shop", []string{"start", "bbbbbbbbbbbb"}},
		{"restart", "project:shop", []string{"restart", "aaaaaaaaaaaa", "bbbbbbbbbbbb"}},
		{"start", "cccccccccccc", []string{"start", "cccccccccccc"}},
	} {
		be.actions = nil
		if err := m.ContainerAction("Ubuntu", tc.action, tc.target); err != nil {
			t.Fatalf("%s %s: %v", tc.action, tc.target, err)
		}
		if len(be.actions) != 1 || !slices.Equal(be.actions[0], tc.want) {
			t.Errorf("%s %s: got %v want %v", tc.action, tc.target, be.actions, tc.want)
		}
	}
	for _, target := range []string{"dddddddddddd", "project:other", "project:"} {
		if err := m.ContainerAction("Ubuntu", "stop", target); err == nil {
			t.Errorf("unknown target %q accepted", target)
		}
	}
	if err := m.ContainerAction("Debian", "stop", "aaaaaaaaaaaa"); err == nil {
		t.Error("action in stopped distro accepted")
	}
}
