package wsl

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotInstalled is returned when no known unit/init script exists for a
// service inside the distro.
var ErrNotInstalled = errors.New("not installed in this distribution")

// serviceScript starts or stops the first unit that exists, using systemd
// when it is PID 1 and SysV `service` otherwise. $1 is the action, the
// remaining args are candidate unit names. Exit 3 means none exists.
const serviceScript = `a=$1; shift; ` +
	`if [ "$(cat /proc/1/comm)" = systemd ]; then ` +
	`for u in "$@"; do if systemctl cat "$u.service" >/dev/null 2>&1; then exec systemctl "$a" "$u.service"; fi; done; ` +
	`else for u in "$@"; do if [ -x "/etc/init.d/$u" ]; then exec service "$u" "$a"; fi; done; fi; ` +
	`exit 3`

// dropCachesScript releases the page cache and compacts memory so WSL's free
// page reporting can hand the RAM back to Windows.
const dropCachesScript = `sync; echo 3 > /proc/sys/vm/drop_caches; ` +
	`if [ -w /proc/sys/vm/compact_memory ]; then echo 1 > /proc/sys/vm/compact_memory; fi`

// rootExec runs a fixed script as root inside a running distro. Only
// constant scripts are passed; dynamic values go in args.
func (r *Runner) rootExec(ctx context.Context, timeout time.Duration, distro, script string, args ...string) (string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	full := append([]string{"--distribution", distro, "--user", "root", "--exec", "/bin/sh", "-c", script, "sh"}, args...)
	cmd := r.command(ctx, full...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		code := -1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		}
		msg := strings.TrimSpace(decodeOutput([]byte(stderr.String())))
		if msg == "" {
			msg = err.Error()
		}
		return decodeOutput(out), code, errors.New(lastLine(msg))
	}
	return decodeOutput(out), 0, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// ServiceControl runs action ("start" or "stop") on the first existing unit.
func (r *Runner) ServiceControl(ctx context.Context, distro, action string, units []string) error {
	if action != "start" && action != "stop" {
		return fmt.Errorf("unsupported action %q", action)
	}
	_, code, err := r.rootExec(ctx, 90*time.Second, distro, serviceScript, append([]string{action}, units...)...)
	if code == 3 {
		return ErrNotInstalled
	}
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

// DropCaches frees Linux's page cache.
func (r *Runner) DropCaches(ctx context.Context, distro string) error {
	_, _, err := r.rootExec(ctx, 60*time.Second, distro, dropCachesScript)
	return err
}

// Trim discards unused filesystem blocks so the VHDX can shrink, returning
// fstrim's summary line (e.g. "/: 1.2 GiB (1288490188 bytes) trimmed").
func (r *Runner) Trim(ctx context.Context, distro string) (string, error) {
	out, _, err := r.rootExec(ctx, 10*time.Minute, distro, `fstrim -v /`)
	if err != nil {
		return "", fmt.Errorf("trim: %w", err)
	}
	return lastLine(out), nil
}

// UNCPath is the Windows path of a guest path, e.g. \\wsl.localhost\Ubuntu\home\me.
func UNCPath(distro, guestPath string) string {
	p := `\\wsl.localhost\` + distro
	if guestPath != "" && guestPath != "/" {
		p += strings.ReplaceAll(guestPath, "/", `\`)
	}
	return p
}

// OpenFiles opens the guest's home (or root when unknown) in File Explorer.
// Accessing \\wsl.localhost boots a stopped distro; this is a user action.
func (r *Runner) OpenFiles(distro, home string) error {
	cmd := exec.Command("explorer.exe", UNCPath(distro, home))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open File Explorer: %w", err)
	}
	go func() { _ = cmd.Wait() }() // explorer.exe exits non-zero routinely
	return nil
}

// ErrNoVSCode is returned when the `code` CLI cannot be found.
var ErrNoVSCode = errors.New("VS Code was not found. Install it, or add its `code` command to PATH")

// OpenVSCode opens VS Code connected to the distro (Remote - WSL), in the
// user's home folder when known.
func (r *Runner) OpenVSCode(distro, home string) error {
	code, err := findVSCode()
	if err != nil {
		return err
	}
	args := []string{"--remote", "wsl+" + distro}
	if home != "" {
		args = []string{"--folder-uri", "vscode-remote://wsl+" + distro + (&url.URL{Path: home}).EscapedPath()}
	}
	cmd := exec.Command(code, args...)
	hideConsole(cmd) // code.cmd would otherwise flash a console
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start VS Code: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func findVSCode() (string, error) {
	if p, err := exec.LookPath("code"); err == nil {
		return p, nil
	}
	for _, base := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("ProgramFiles")} {
		if base == "" {
			continue
		}
		dir := filepath.Join(base, "Microsoft VS Code", "bin")
		if base == os.Getenv("LOCALAPPDATA") {
			dir = filepath.Join(base, "Programs", "Microsoft VS Code", "bin")
		}
		if p := filepath.Join(dir, "code.cmd"); fileExists(p) {
			return p, nil
		}
	}
	return "", ErrNoVSCode
}
