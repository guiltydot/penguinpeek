// Package monitor owns the single polling loop. It decides how often to talk
// to wsl.exe based on window visibility, caches the latest snapshot, and
// pushes updates through a callback instead of being polled.
package monitor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"penguinpeek/internal/wsl"
)

const (
	// HiddenInterval is used while no window is visible: the running list is
	// refreshed for the tray, plus one guest sample when alerts need it.
	HiddenInterval = 30 * time.Second
	// fullEvery is how often the slow-changing guest facts (disk usage,
	// $HOME, os-release) and the VHDX size are re-read while visible.
	fullEvery = 30 * time.Second
	// cpuSampleMaxAge: older previous samples are discarded and the guest
	// script samples twice instead.
	cpuSampleMaxAge = 15 * time.Second
	// quietFor is how long a stop the user asked for is not reported as
	// unexpected.
	quietFor = 2 * time.Minute
)

// Backend is the subset of wsl.Runner the monitor needs (fakeable in tests).
type Backend interface {
	List(ctx context.Context) ([]wsl.Distro, error)
	Running(ctx context.Context) (map[string]bool, error)
	Metrics(ctx context.Context, distro string, twoSamples bool, level wsl.Level) (wsl.Sample, error)
	KeepAlive(distro string) (*exec.Cmd, error)
	Terminate(ctx context.Context, distro string) error
	Shutdown(ctx context.Context) error
	OpenTerminal(distro string) error
	ServiceControl(ctx context.Context, distro, action string, units []string) error
	DropCaches(ctx context.Context, distro string) error
	Trim(ctx context.Context, distro string) (string, error)
	OpenFiles(distro, home string) error
	OpenVSCode(distro, home string) error
	Containers(ctx context.Context, distro string) ([]wsl.Container, error)
	ContainerAction(ctx context.Context, distro, action string, ids []string) error
}

// Host is the Windows side (fakeable in tests; stubs on Linux).
type Host interface {
	VMMemory() (uint64, error)
	VHDXPath(distro string) (string, error)
	FileSize(path string) (uint64, error)
	CompactVHDX(path string) error
}

// DiskInfo combines the guest filesystem with the VHDX file on Windows.
type DiskInfo struct {
	Used  uint64 `json:"used"`  // used inside Linux
	Total uint64 `json:"total"` // filesystem capacity
	File  uint64 `json:"file"`  // ext4.vhdx size on the Windows drive; 0 if unknown
	Path  string `json:"path"`
}

// Metrics for the selected distro. CPU and memory are VM-wide because all
// WSL2 distros share one utility VM.
type Metrics struct {
	CPUPercent float64    `json:"cpuPercent"`
	Memory     wsl.Memory `json:"memory"`
	// HostMemory is the VM's working set on Windows (vmmemWSL); 0 if unknown.
	HostMemory uint64          `json:"hostMemory"`
	UptimeSec  float64         `json:"uptimeSec"`
	Services   []ServiceStatus `json:"services"`
	Ports      []wsl.Port      `json:"ports"`
	Disk       DiskInfo        `json:"disk"`
	Home       string          `json:"home"`
	OSID       string          `json:"osId"`   // /etc/os-release ID
	OSLike     string          `json:"osLike"` // /etc/os-release ID_LIKE
	// Containers is only filled while the Containers tab is open; nil
	// means it was not requested (or Docker is not running).
	Containers      []wsl.Container `json:"containers"`
	ContainersError string          `json:"containersError"`
	SampledAt       time.Time       `json:"sampledAt"`
}

// Snapshot is everything the UI renders.
type Snapshot struct {
	Distros   []wsl.Distro `json:"distros"`
	Selected  string       `json:"selected"`
	Running   bool         `json:"running"`
	Busy      string       `json:"busy"` // "", "starting", "stopping", "restarting", "shutting down"
	Metrics   *Metrics     `json:"metrics"`
	Error     string       `json:"error"`
	Live      bool         `json:"live"` // metrics are from the current visible tick
	UpdatedAt time.Time    `json:"updatedAt"`
	// Quiet lists "distro" and "distro/serviceID" keys the user stopped
	// recently, so alerts can skip them. Not sent to the frontend.
	Quiet []string `json:"-"`
}

// Monitor is safe for concurrent use.
type Monitor struct {
	be       Backend
	host     Host
	onUpdate func(Snapshot)
	wake     chan struct{}

	mu          sync.Mutex
	snap        Snapshot
	visible     bool
	probeHidden bool
	interval    time.Duration
	watched     []string
	// containersWanted: the Containers tab is open, so visible ticks also
	// list Docker containers (one extra wsl.exe call).
	containersWanted bool
	// containerNames maps container IDs to names from the last listing, so
	// services found inside a container can say which one. namesAt is when
	// a listing was last made just to learn a missing name.
	containerNames map[string]string
	namesAt        time.Time
	// The registered distros are listed once (or loaded from settings) and
	// then only re-listed on request or when an unknown one starts running;
	// every tick asks wsl.exe which are running.
	listWanted  bool
	wsl1        map[string]bool // registered WSL1 names, ignored
	saveDistros func([]wsl.Distro)
	// full holds the last LevelFull values, carried into lighter samples.
	full     fullInfo
	fullAt   time.Time
	fullName string
	prevCPU  wsl.CPUSample
	prevAt   time.Time
	prevName string
	keep     map[string]*exec.Cmd
	quiet    map[string]time.Time
	vhdx     map[string]string
}

// fullInfo is what a LevelFull sample adds, plus the VHDX size.
type fullInfo struct {
	disk         DiskInfo
	home         string
	osID, osLike string
	tailscaleSSH bool
}

// New creates a monitor. onUpdate is called from the loop goroutine.
func New(be Backend, host Host, selected string, interval time.Duration, watched []string, onUpdate func(Snapshot)) *Monitor {
	return &Monitor{
		be:       be,
		host:     host,
		onUpdate: onUpdate,
		wake:     make(chan struct{}, 1),
		snap:     Snapshot{Selected: selected},
		interval: interval,
		watched:  watched,
		keep:     map[string]*exec.Cmd{},
		quiet:    map[string]time.Time{},
		vhdx:     map[string]string{},
	}
}

// UseKnownDistros seeds the distro list (e.g. from settings) so startup needs
// no `wsl --list`, and registers save to persist the list whenever a fresh
// listing differs. Call before Run.
func (m *Monitor) UseKnownDistros(ds []wsl.Distro, save func([]wsl.Distro)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.Distros = slices.Clone(ds)
	m.saveDistros = save
}

// RefreshDistros re-lists the registered distros on the next tick (e.g. the
// user installed or removed one).
func (m *Monitor) RefreshDistros() {
	m.mu.Lock()
	m.listWanted = true
	m.mu.Unlock()
	m.Refresh()
}

// Run blocks until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) {
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			m.releaseKeepAlives()
			return
		case <-t.C:
		case <-m.wake:
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
		}
		m.tick(ctx)
		t.Reset(m.nextDelay())
	}
}

func (m *Monitor) nextDelay() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.visible {
		return HiddenInterval
	}
	return m.interval
}

// Refresh schedules an immediate tick.
func (m *Monitor) Refresh() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// SetVisible switches between the cheap hidden cadence and live metrics.
func (m *Monitor) SetVisible(v bool) {
	m.mu.Lock()
	changed := m.visible != v
	m.visible = v
	m.mu.Unlock()
	if changed && v {
		m.Refresh()
	}
}

// SetProbeHidden makes hidden ticks also sample the selected running distro
// (one wsl.exe call per 30 s), so service and memory alerts work while the
// flyout is closed. It never runs anything in a stopped distro.
func (m *Monitor) SetProbeHidden(on bool) {
	m.mu.Lock()
	m.probeHidden = on
	m.mu.Unlock()
}

// SetInterval sets the visible refresh interval.
func (m *Monitor) SetInterval(d time.Duration) {
	m.mu.Lock()
	m.interval = d
	m.mu.Unlock()
	m.Refresh()
}

// SetWatched sets which dev services are always listed.
func (m *Monitor) SetWatched(ids []string) {
	m.mu.Lock()
	m.watched = ids
	m.mu.Unlock()
	m.Refresh()
}

// SetContainersWanted turns container listing on visible ticks on or off.
func (m *Monitor) SetContainersWanted(on bool) {
	m.mu.Lock()
	changed := m.containersWanted != on
	m.containersWanted = on
	m.mu.Unlock()
	if changed && on {
		m.Refresh()
	}
}

// Select changes the distro the UI is focused on.
func (m *Monitor) Select(name string) {
	m.mu.Lock()
	if m.snap.Selected != name {
		m.snap.Selected = name
		m.snap.Metrics = nil
		m.snap.Live = false
	}
	m.mu.Unlock()
	m.Refresh()
}

// Snapshot returns the cached snapshot.
func (m *Monitor) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snap
}

func (m *Monitor) tick(ctx context.Context) {
	m.mu.Lock()
	visible := m.visible
	probe := visible || m.probeHidden
	needList := m.listWanted || len(m.snap.Distros) == 0
	m.listWanted = false
	distros := slices.Clone(m.snap.Distros)
	m.mu.Unlock()

	var errMsg string
	if !needList {
		running, _ := m.be.Running(ctx)
		for i := range distros {
			distros[i].Running = running[distros[i].Name]
		}
		m.mu.Lock()
		for name := range running {
			if !m.wsl1[name] && !slices.ContainsFunc(distros, func(d wsl.Distro) bool { return d.Name == name }) {
				needList = true // a distro we have not seen: installed since
			}
		}
		m.mu.Unlock()
	}
	if needList {
		var ok bool
		distros, ok = m.listDistros(ctx)
		if !ok {
			errMsg = "WSL is not available or no distributions are installed."
		}
	}

	m.mu.Lock()
	selected := pickSelected(m.snap.Selected, distros)
	var sel *wsl.Distro
	for i := range distros {
		if distros[i].Name == selected {
			sel = &distros[i]
		}
	}
	running := sel != nil && sel.Running
	twoSamples := visible && (m.prevName != selected || time.Since(m.prevAt) > cpuSampleMaxAge)
	prev, prevName := m.prevCPU, m.prevName
	watched := slices.Clone(m.watched)
	wantContainers := visible && m.containersWanted
	m.mu.Unlock()

	m.mu.Lock()
	level := wsl.LevelLite
	if visible {
		level = wsl.LevelLive
		if m.fullName != selected || time.Since(m.fullAt) > fullEvery {
			level = wsl.LevelFull
		}
	}
	full, haveFull := m.full, m.fullName == selected
	var prevPorts []wsl.Port
	if m.snap.Metrics != nil && m.snap.Selected == selected {
		prevPorts = m.snap.Metrics.Ports
	}
	m.mu.Unlock()

	var metrics *Metrics
	sampled := false
	if probe && running {
		s, err := m.be.Metrics(ctx, selected, twoSamples, level)
		if err == nil {
			cur := s.CPU[len(s.CPU)-1]
			base := prev
			if len(s.CPU) > 1 {
				base = s.CPU[0]
			} else if prevName != selected {
				base = cur // no baseline yet: report 0 rather than boot-time average
			}
			if level == wsl.LevelFull {
				full = fullInfo{
					disk: DiskInfo{Used: s.Disk.Used, Total: s.Disk.Total},
					home: s.Home, osID: s.OSID, osLike: s.OSLike, tailscaleSSH: s.TailscaleSSH,
				}
				m.addDiskFile(&full.disk, selected)
				haveFull = true
				m.mu.Lock()
				m.full, m.fullAt, m.fullName = full, time.Now(), selected
				m.mu.Unlock()
			}
			if level == wsl.LevelLite {
				s.Ports = prevPorts // not sampled while hidden
			}
			guest := func() guestState {
				return guestState{s.Procs, s.ContainerProcs, m.knownContainers(), s.Ports, haveFull && full.tailscaleSSH}
			}
			metrics = &Metrics{
				CPUPercent: wsl.CPUPercent(base, cur),
				Memory:     s.Mem,
				UptimeSec:  s.UptimeSec,
				Services:   detectServices(guest(), distros, watched),
				Ports:      s.Ports,
				SampledAt:  time.Now(),
			}
			// List containers for the Containers tab, or (at most every
			// namesEvery) to name the container a service runs in.
			lookup := visible && unnamedContainer(metrics.Services) && m.namesDue()
			if (wantContainers || lookup) && dockerRunning(metrics.Services) {
				cs, cerr := m.listContainers(ctx, selected)
				if cerr == "" {
					m.rememberContainers(cs)
					metrics.Services = detectServices(guest(), distros, watched)
				}
				if wantContainers {
					metrics.Containers, metrics.ContainersError = cs, cerr
				}
			}
			if haveFull {
				metrics.Disk, metrics.Home, metrics.OSID, metrics.OSLike = full.disk, full.home, full.osID, full.osLike
			}
			if visible && m.host != nil {
				if ws, err := m.host.VMMemory(); err == nil {
					metrics.HostMemory = ws
				}
			}
			sampled = true
			m.mu.Lock()
			m.prevCPU, m.prevAt, m.prevName = cur, time.Now(), selected
			m.mu.Unlock()
		} else if !errors.Is(err, context.Canceled) && visible {
			errMsg = "Could not read metrics from " + selected + "."
		}
	}

	m.mu.Lock()
	if metrics == nil && running && m.snap.Selected == selected {
		metrics = m.snap.Metrics // keep last values while hidden
	}
	next := Snapshot{
		Distros:   distros,
		Selected:  selected,
		Running:   running,
		Busy:      m.snap.Busy,
		Metrics:   metrics,
		Error:     errMsg,
		Live:      sampled && visible,
		UpdatedAt: time.Now(),
		Quiet:     m.quietKeysLocked(),
	}
	changed := sampled || !sameState(m.snap, next)
	m.snap = next
	m.mu.Unlock()

	if changed && m.onUpdate != nil {
		m.onUpdate(next)
	}
}

func dockerRunning(ss []ServiceStatus) bool {
	return slices.ContainsFunc(ss, func(s ServiceStatus) bool { return s.ID == "docker" && s.Running })
}

// namesEvery limits container listings made only to name a service's
// container (the name is cached, so this matters only while it is unknown).
const namesEvery = 30 * time.Second

func (m *Monitor) knownContainers() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.containerNames
}

// namesDue reports whether a name lookup may run now, and marks it as run.
func (m *Monitor) namesDue() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.namesAt) < namesEvery {
		return false
	}
	m.namesAt = time.Now()
	return true
}

func (m *Monitor) rememberContainers(cs []wsl.Container) {
	names := make(map[string]string, len(cs))
	for _, c := range cs {
		names[c.ID] = c.Name
	}
	m.mu.Lock()
	m.containerNames = names
	m.mu.Unlock()
}

// listContainers returns the containers, or a user-facing reason why not.
func (m *Monitor) listContainers(ctx context.Context, distro string) ([]wsl.Container, string) {
	cs, err := m.be.Containers(ctx, distro)
	switch {
	case err == nil:
		if cs == nil {
			cs = []wsl.Container{} // listed, none: distinct from "not requested"
		}
		return cs, ""
	case errors.Is(err, wsl.ErrNoDockerCLI):
		return nil, "The docker command was not found in " + distro +
			". With Docker Desktop, turn on WSL integration for this distribution."
	case errors.Is(err, wsl.ErrDockerUnavailable):
		return nil, "Docker is not responding in " + distro + "."
	default:
		return nil, "Could not list containers: " + err.Error()
	}
}

// addDiskFile fills in the distro's VHDX path and size on Windows.
func (m *Monitor) addDiskFile(d *DiskInfo, distro string) {
	if m.host == nil {
		return
	}
	m.mu.Lock()
	path, ok := m.vhdx[distro]
	m.mu.Unlock()
	if !ok {
		p, err := m.host.VHDXPath(distro)
		if err != nil {
			p = ""
		}
		path = p
		m.mu.Lock()
		m.vhdx[distro] = p
		m.mu.Unlock()
	}
	// Only report a disk file that exists (e.g. it may have been moved).
	if path != "" {
		if size, err := m.host.FileSize(path); err == nil {
			d.Path, d.File = path, size
		}
	}
}

// listDistros runs the full `wsl --list` and persists the result if the set
// of registered distros changed.
func (m *Monitor) listDistros(ctx context.Context) ([]wsl.Distro, bool) {
	ds, err := m.be.List(ctx)
	// PenguinPeek is WSL2-only: WSL1 distros have no VM, VHDX or shared
	// kernel to monitor, so they are left out entirely.
	wsl1 := map[string]bool{}
	ds = slices.DeleteFunc(ds, func(d wsl.Distro) bool {
		if d.Version == 1 {
			wsl1[d.Name] = true
		}
		return d.Version == 1
	})
	if err != nil && len(ds) == 0 {
		m.mu.Lock()
		old := slices.Clone(m.snap.Distros)
		m.mu.Unlock()
		return old, len(old) > 0 // keep a cached list through a wsl.exe hiccup
	}
	m.mu.Lock()
	m.wsl1 = wsl1
	changed := !sameRegistered(m.snap.Distros, ds)
	save := m.saveDistros
	m.mu.Unlock()
	if changed && save != nil {
		save(registered(ds))
	}
	return ds, len(ds) > 0
}

// registered strips the running state, which is not worth persisting.
func registered(ds []wsl.Distro) []wsl.Distro {
	out := slices.Clone(ds)
	for i := range out {
		out[i].Running = false
	}
	return out
}

func sameRegistered(a, b []wsl.Distro) bool {
	return slices.Equal(registered(a), registered(b))
}

func sameState(a, b Snapshot) bool {
	return a.Selected == b.Selected && a.Running == b.Running && a.Busy == b.Busy &&
		a.Error == b.Error && reflect.DeepEqual(a.Distros, b.Distros)
}

func pickSelected(cur string, ds []wsl.Distro) string {
	for _, d := range ds {
		if d.Name == cur {
			return cur
		}
	}
	for _, d := range ds {
		if d.Default {
			return d.Name
		}
	}
	if len(ds) > 0 {
		return ds[0].Name
	}
	return cur
}

func (m *Monitor) markQuiet(keys ...string) {
	m.mu.Lock()
	for _, k := range keys {
		m.quiet[k] = time.Now()
	}
	m.mu.Unlock()
}

func (m *Monitor) markAllRunningQuiet() {
	m.mu.Lock()
	for _, d := range m.snap.Distros {
		if d.Running {
			m.quiet[d.Name] = time.Now()
		}
	}
	m.mu.Unlock()
}

func (m *Monitor) quietKeysLocked() []string {
	var keys []string
	for k, at := range m.quiet {
		if time.Since(at) > quietFor {
			delete(m.quiet, k)
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

func (m *Monitor) setBusy(b string) {
	m.mu.Lock()
	m.snap.Busy = b
	s := m.snap
	m.mu.Unlock()
	if m.onUpdate != nil {
		m.onUpdate(s)
	}
}

// do runs an action with a busy indicator, then refreshes.
func (m *Monitor) do(busy string, f func() error) error {
	m.setBusy(busy)
	err := f()
	m.setBusy("")
	m.Refresh()
	return err
}

// Start boots a distro and keeps it running while PenguinPeek runs.
func (m *Monitor) Start(name string) error {
	return m.do("starting", func() error {
		err := m.start(name)
		if err != nil {
			m.mu.Lock()
			m.listWanted = true // it may have been unregistered meanwhile
			m.mu.Unlock()
		}
		return err
	})
}

// refreshFull makes the next tick re-read disk usage and the VHDX size.
func (m *Monitor) refreshFull() {
	m.mu.Lock()
	m.fullName = ""
	m.mu.Unlock()
	m.Refresh()
}

func (m *Monitor) start(name string) error {
	m.mu.Lock()
	old := m.keep[name]
	m.mu.Unlock()
	if old != nil && old.ProcessState == nil {
		return nil // our keep-alive is still running
	}
	cmd, err := m.be.KeepAlive(name)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.keep[name] = cmd
	m.mu.Unlock()
	// Wait until wsl.exe reports it running (boot takes ~1-3 s).
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for {
		if r, _ := m.be.Running(ctx); r[name] {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New(name + " did not start in time")
		case <-time.After(400 * time.Millisecond):
		}
	}
}

// Stop terminates a distro.
func (m *Monitor) Stop(name string) error {
	return m.do("stopping", func() error { return m.stop(name) })
}

func (m *Monitor) stop(name string) error {
	m.markQuiet(name)
	m.dropKeepAlive(name)
	return m.be.Terminate(context.Background(), name)
}

// Restart terminates and boots a distro.
func (m *Monitor) Restart(name string) error {
	return m.do("restarting", func() error {
		if err := m.stop(name); err != nil {
			return err
		}
		return m.start(name)
	})
}

// ShutdownAll stops the WSL VM and every distro.
func (m *Monitor) ShutdownAll() error {
	return m.do("shutting down", func() error {
		m.markAllRunningQuiet()
		m.releaseKeepAlives()
		return m.be.Shutdown(context.Background())
	})
}

// OpenTerminal opens a shell in the distro (booting it, as the user asked).
func (m *Monitor) OpenTerminal(name string) error {
	err := m.be.OpenTerminal(name)
	go func() { time.Sleep(3 * time.Second); m.Refresh() }()
	return err
}

// home returns the cached $HOME for a distro, if it was sampled.
func (m *Monitor) home(name string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.snap.Selected == name && m.snap.Metrics != nil {
		return m.snap.Metrics.Home
	}
	return ""
}

// OpenFiles opens the distro's home folder in File Explorer.
func (m *Monitor) OpenFiles(name string) error { return m.be.OpenFiles(name, m.home(name)) }

// OpenVSCode opens VS Code connected to the distro.
func (m *Monitor) OpenVSCode(name string) error { return m.be.OpenVSCode(name, m.home(name)) }

// ErrNotRunning is returned by actions that need a running distro.
var ErrNotRunning = errors.New("the distribution is not running")

func (m *Monitor) requireRunning(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.snap.Distros {
		if d.Name == name && d.Running {
			return nil
		}
	}
	return ErrNotRunning
}

// ServiceControl starts or stops a known dev service as root.
func (m *Monitor) ServiceControl(name, id, action string) error {
	i := slices.IndexFunc(Catalog, func(d ServiceDef) bool { return d.ID == id })
	if i < 0 {
		return fmt.Errorf("unknown service %q", id)
	}
	def := Catalog[i]
	if len(def.Units) == 0 {
		return fmt.Errorf("%s cannot be started or stopped from PenguinPeek", def.Name)
	}
	if err := m.requireRunning(name); err != nil {
		return err
	}
	if action == "stop" {
		m.markQuiet(name + "/" + id)
	}
	err := m.be.ServiceControl(context.Background(), name, action, def.Units)
	m.Refresh()
	if errors.Is(err, wsl.ErrNotInstalled) {
		return fmt.Errorf("%s is not installed in %s", def.Name, name)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", def.Name, err)
	}
	return nil
}

// ContainerAction starts, stops or restarts a container, or a whole compose
// project when target is "project:<name>". Targets must come from the last
// listing, so only containers the UI was shown can be acted on.
func (m *Monitor) ContainerAction(name, action, target string) error {
	if err := m.requireRunning(name); err != nil {
		return err
	}
	m.mu.Lock()
	var cs []wsl.Container
	if m.snap.Selected == name && m.snap.Metrics != nil {
		cs = m.snap.Metrics.Containers
	}
	m.mu.Unlock()
	ids, err := containerTargets(cs, action, target)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil // e.g. starting a project that is already fully up
	}
	err = m.be.ContainerAction(context.Background(), name, action, ids)
	m.Refresh()
	return err
}

func containerTargets(cs []wsl.Container, action, target string) ([]string, error) {
	if project, ok := strings.CutPrefix(target, "project:"); ok {
		if action == "restart" {
			action = "" // every container of the project
		}
		var ids []string
		found := false
		for _, c := range cs {
			if c.Project != project || project == "" {
				continue
			}
			found = true
			if action == "" || (action == "stop") == c.Running() {
				ids = append(ids, c.ID)
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown compose project %q", project)
		}
		return ids, nil
	}
	i := slices.IndexFunc(cs, func(c wsl.Container) bool { return c.ID == target })
	if i < 0 {
		return nil, errors.New("that container is no longer listed; refresh and try again")
	}
	return []string{cs[i].ID}, nil
}

// FreeMemory drops Linux's page cache so Windows can reclaim the RAM.
func (m *Monitor) FreeMemory(name string) error {
	if err := m.requireRunning(name); err != nil {
		return err
	}
	err := m.be.DropCaches(context.Background(), name)
	m.Refresh()
	return err
}

// Trim discards unused blocks in the distro's filesystem.
func (m *Monitor) Trim(name string) (string, error) {
	if err := m.requireRunning(name); err != nil {
		return "", err
	}
	out, err := m.be.Trim(context.Background(), name)
	m.refreshFull()
	return out, err
}

// CompactDisk shuts WSL down and compacts the distro's VHDX in an elevated
// window (UAC prompt).
func (m *Monitor) CompactDisk(name string) error {
	if m.host == nil {
		return errors.New("not supported")
	}
	path, err := m.host.VHDXPath(name)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".vhdx") {
		return fmt.Errorf("unexpected disk file %q", path)
	}
	m.markAllRunningQuiet()
	m.releaseKeepAlives()
	m.mu.Lock()
	m.fullName = "" // re-read the file size once WSL is back
	m.mu.Unlock()
	return m.host.CompactVHDX(path)
}

func (m *Monitor) dropKeepAlive(name string) {
	m.mu.Lock()
	cmd := m.keep[name]
	delete(m.keep, name)
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// releaseKeepAlives kills our hidden keep-alive processes. Distros then
// follow WSL's normal idle shutdown, so nothing lingers after Quit.
func (m *Monitor) releaseKeepAlives() {
	m.mu.Lock()
	names := make([]string, 0, len(m.keep))
	for n := range m.keep {
		names = append(names, n)
	}
	m.mu.Unlock()
	for _, n := range names {
		m.dropKeepAlive(n)
	}
}
