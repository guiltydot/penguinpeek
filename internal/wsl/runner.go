package wsl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Level selects how much a guest sample collects.
type Level int

const (
	// LevelLite: uptime, CPU, memory and process names (alerts while hidden).
	LevelLite Level = iota
	// LevelLive adds listening ports (every visible tick).
	LevelLive
	// LevelFull adds disk usage, $HOME, os-release, PID 1 and whether
	// Tailscale SSH is on, which rarely change and are refreshed every 30 s.
	LevelFull
)

// metricsScript runs inside the guest as one sh invocation per tick. It is a
// single line because wsl.exe forwards --exec arguments through the Windows
// command line. $1 = 2 asks for a second /proc/stat sample 200 ms later, used
// when there is no recent previous sample to diff against; $2 is the Level.
// The always-on sections use shell builtins only, so a lite or live sample
// forks nothing but ss. Skipped sections stay in place (empty) so the
// "@@"-separated layout is fixed.
const metricsScript = `read -r up _ < /proc/uptime; echo "$up"; echo @@; ` +
	`read -r l < /proc/stat; echo "$l"; ` +
	`if [ "$1" = 2 ]; then sleep 0.2; read -r l < /proc/stat; echo "$l"; fi; echo @@; ` +
	`while read -r k v _; do case $k in MemTotal:|MemAvailable:|SwapTotal:|SwapFree:) echo "$k $v kB";; esac; done < /proc/meminfo; echo @@; ` +
	// Processes inside a Docker/Podman container are printed as
	// "%<cgroup> <comm>" so a containerized nginx is not taken for the
	// distro's own nginx service.
	`{ for d in /proc/[0-9]*; do read -r c < "$d/comm" || continue; g=; read -r g < "$d/cgroup"; ` +
	`case $g in *docker[-/]*|*libpod-*) echo "%$g $c";; *) echo "$c"; [ "$c" = tailscaled ] && t=1;; esac; done; } 2>/dev/null; echo @@; ` +
	`if [ "$2" = 2 ]; then df -kP /; fi; echo @@; ` +
	// ss names only the caller's own processes without root; that covers
	// dev servers, and the UI labels well-known ports for the rest.
	`if [ "$2" -ge 1 ]; then if command -v ss >/dev/null 2>&1; then ss -ltnpH 2>/dev/null; ` +
	`else awk 'FNR>1 && $4=="0A" {print "P", $2}' /proc/net/tcp /proc/net/tcp6 2>/dev/null; fi; fi; echo @@; ` +
	`if [ "$2" = 2 ]; then echo "$HOME"; ( . /etc/os-release 2>/dev/null; echo "os:$ID:$ID_LIKE" ); read -r i < /proc/1/comm; echo "$i"; fi; echo @@; ` +
	// Tailscale SSH runs inside tailscaled, so it has no process or port of
	// its own; ask tailscaled (its socket is world-writable) at full level.
	`if [ "$2" = 2 ] && [ "$t" = 1 ] && command -v tailscale >/dev/null 2>&1; then ` +
	`case "$(tailscale debug prefs 2>/dev/null)" in *'"RunSSH": true'*) echo tsssh;; esac; fi`

// Runner invokes wsl.exe.
type Runner struct {
	exe string
}

// NewRunner finds wsl.exe. On Linux (development inside WSL) it uses interop.
func NewRunner() *Runner {
	exe := "wsl.exe"
	if runtime.GOOS != "windows" {
		if p := "/mnt/c/Windows/System32/wsl.exe"; fileExists(p) {
			exe = p
		}
	}
	return &Runner{exe: exe}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (r *Runner) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, r.exe, args...)
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	hideConsole(cmd)
	return cmd
}

func (r *Runner) output(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := r.command(ctx, args...).Output()
	s := decodeOutput(out)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			s = decodeOutput(ee.Stderr)
		}
		return s, fmt.Errorf("wsl.exe %s: %w", strings.Join(args, " "), err)
	}
	return s, nil
}

// List returns all registered distros with their running state.
func (r *Runner) List(ctx context.Context) ([]Distro, error) {
	out, err := r.output(ctx, 10*time.Second, "--list", "--verbose")
	if err != nil {
		// No distros installed also exits non-zero; treat as empty.
		return nil, err
	}
	ds := parseListVerbose(out)
	running, err := r.Running(ctx)
	if err != nil {
		return ds, err
	}
	for i := range ds {
		ds[i].Running = running[ds[i].Name]
	}
	return ds, nil
}

// Running returns the set of running distro names. This is the cheap call
// used while the window is hidden.
func (r *Runner) Running(ctx context.Context) (map[string]bool, error) {
	out, err := r.output(ctx, 10*time.Second, "--list", "--running", "--quiet")
	set := map[string]bool{}
	if err != nil {
		// Exits non-zero when nothing is running.
		return set, nil
	}
	for _, n := range parseNames(out) {
		set[n] = true
	}
	return set, nil
}

// Metrics samples guest metrics. The caller must only pass a running distro;
// executing in a stopped one would boot it.
func (r *Runner) Metrics(ctx context.Context, distro string, twoSamples bool, level Level) (Sample, error) {
	mode := "1"
	if twoSamples {
		mode = "2"
	}
	out, err := r.output(ctx, 8*time.Second,
		"--distribution", distro, "--exec", "/bin/sh", "-c", metricsScript, "sh", mode, strconv.Itoa(int(level)))
	if err != nil {
		return Sample{}, err
	}
	s := ParseSample(out)
	if len(s.CPU) == 0 || s.Mem.Total == 0 {
		return s, fmt.Errorf("unexpected metrics output from %s", distro)
	}
	return s, nil
}

// KeepAlive boots the distro and keeps it running with a hidden
// `sleep infinity`; WSL would otherwise stop it again once idle. Kill the
// returned process (or terminate the distro) to release it.
func (r *Runner) KeepAlive(distro string) (*exec.Cmd, error) {
	cmd := r.command(context.Background(), "--distribution", distro, "--exec", "sleep", "infinity")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", distro, err)
	}
	go func() { _ = cmd.Wait() }()
	return cmd, nil
}

// Terminate stops one distro.
func (r *Runner) Terminate(ctx context.Context, distro string) error {
	_, err := r.output(ctx, 30*time.Second, "--terminate", distro)
	return err
}

// Shutdown stops the whole WSL VM.
func (r *Runner) Shutdown(ctx context.Context) error {
	_, err := r.output(ctx, 60*time.Second, "--shutdown")
	return err
}

// OpenTerminal opens an interactive shell in Windows Terminal, falling back
// to a plain console window.
func (r *Runner) OpenTerminal(distro string) error {
	if wt, err := exec.LookPath("wt.exe"); err == nil {
		cmd := exec.Command(wt, "new-tab", "--title", distro, "wsl.exe", "--distribution", distro, "--cd", "~")
		if err := cmd.Start(); err == nil {
			go func() { _ = cmd.Wait() }()
			return nil
		}
	}
	cmd := exec.Command(r.exe, "--distribution", distro, "--cd", "~")
	newConsole(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open terminal for %s: %w", distro, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
