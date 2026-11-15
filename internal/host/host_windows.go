// Package host reads and changes Windows-side state: the WSL VM's memory as
// Windows sees it, distro disk files, launch-at-login, and disk compaction.
package host

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// VMMemory returns the working set of the WSL utility VM process
// (vmmemWSL, or vmmem on older builds): the RAM WSL really costs Windows,
// including Linux's page cache. 0 when WSL's VM is not running.
//
// It reads the system process table like Task Manager does; opening the
// VM process itself is denied even for PROCESS_QUERY_LIMITED_INFORMATION.
func VMMemory() (uint64, error) {
	procBuf.Lock()
	defer procBuf.Unlock()
	if procBuf.b == nil {
		procBuf.b = make([]byte, 512<<10)
	}
	ok := false
	for range 8 {
		var need uint32
		err := windows.NtQuerySystemInformation(windows.SystemProcessInformation,
			unsafe.Pointer(&procBuf.b[0]), uint32(len(procBuf.b)), &need)
		if err == nil {
			ok = true
			break
		}
		if !errors.Is(err, windows.STATUS_INFO_LENGTH_MISMATCH) {
			return 0, fmt.Errorf("query processes: %w", err)
		}
		procBuf.b = make([]byte, max(int(need)+64<<10, 2*len(procBuf.b)))
	}
	if !ok {
		return 0, errors.New("query processes: buffer kept growing")
	}
	buf := procBuf.b
	var total uint64
	for off := uint32(0); ; {
		p := (*windows.SYSTEM_PROCESS_INFORMATION)(unsafe.Pointer(&buf[off]))
		// Only decode names as long as "vmmem", "vmmemWSL" or with ".exe".
		switch p.ImageName.Length / 2 {
		case 5, 8, 9, 12:
			switch strings.ToLower(p.ImageName.String()) {
			case "vmmemwsl", "vmmem", "vmmemwsl.exe", "vmmem.exe":
				total += uint64(p.WorkingSetSize)
			}
		}
		if p.NextEntryOffset == 0 {
			break
		}
		off += p.NextEntryOffset
	}
	return total, nil
}

// procBuf is reused between VMMemory calls: the process table is ~0.5 MB
// and is read on every visible tick.
var procBuf struct {
	sync.Mutex
	b []byte
}

const lxssKey = `Software\Microsoft\Windows\CurrentVersion\Lxss`

// VHDXPath returns the ext4.vhdx of a distro from WSL's registry entries.
func VHDXPath(distro string) (string, error) {
	root, err := registry.OpenKey(registry.CURRENT_USER, lxssKey, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return "", fmt.Errorf("open Lxss key: %w", err)
	}
	defer root.Close()
	ids, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return "", err
	}
	for _, id := range ids {
		k, err := registry.OpenKey(root, id, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		name, _, _ := k.GetStringValue("DistributionName")
		base, _, _ := k.GetStringValue("BasePath")
		vhd, _, verr := k.GetStringValue("VhdFileName")
		k.Close()
		if !strings.EqualFold(name, distro) || base == "" {
			continue
		}
		if verr != nil || vhd == "" {
			vhd = "ext4.vhdx"
		}
		base = strings.TrimPrefix(base, `\\?\`)
		return filepath.Join(base, vhd), nil
	}
	return "", fmt.Errorf("no disk registered for %s", distro)
}

// FileSize returns a file's size in bytes.
func FileSize(path string) (uint64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return uint64(fi.Size()), nil
}

const (
	runKey       = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "PenguinPeek"
)

// AutostartEnabled reports whether PenguinPeek starts at login, and whether
// the entry points at a different exe (e.g. an older install location).
func AutostartEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runValueName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// SetAutostart adds or removes the per-user Run entry for this exe.
func SetAutostart(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(runValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(runValueName, `"`+exe+`"`)
}

// compactScript runs elevated. It shuts WSL down (diskpart cannot compact an
// attached disk), then compacts the VHDX read-only attached.
const compactScript = `param([string]$Vhdx)
$ErrorActionPreference = 'Stop'
$Host.UI.RawUI.WindowTitle = 'PenguinPeek - compact WSL disk'
Write-Host "Compacting $Vhdx"
$before = (Get-Item -LiteralPath $Vhdx).Length
Write-Host 'Shutting down WSL...'
wsl.exe --shutdown
Start-Sleep -Seconds 3
$dp = Join-Path $env:TEMP 'penguinpeek-compact.txt'
@(
  "select vdisk file=""$Vhdx""",
  'attach vdisk readonly',
  'compact vdisk',
  'detach vdisk'
) | Set-Content -Encoding ASCII -LiteralPath $dp
try { diskpart /s $dp } finally { Remove-Item -LiteralPath $dp -ErrorAction SilentlyContinue }
$after = (Get-Item -LiteralPath $Vhdx).Length
Write-Host ('Done: {0:N2} GB -> {1:N2} GB' -f ($before/1GB), ($after/1GB))
Read-Host 'Press Enter to close'
`

// CompactVHDX shows a UAC prompt and runs the compaction in a visible,
// elevated PowerShell window. It returns once the window is launched.
func CompactVHDX(vhdx string) error {
	if _, err := os.Stat(vhdx); err != nil {
		return fmt.Errorf("disk file: %w", err)
	}
	script := filepath.Join(os.TempDir(), "penguinpeek-compact.ps1")
	if err := os.WriteFile(script, []byte(compactScript), 0o600); err != nil {
		return err
	}
	args := fmt.Sprintf(`-NoProfile -ExecutionPolicy Bypass -File "%s" -Vhdx "%s"`, script, vhdx)
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString("powershell.exe")
	params, _ := syscall.UTF16PtrFromString(args)
	if err := windows.ShellExecute(0, verb, file, params, nil, windows.SW_SHOWNORMAL); err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return errors.New("compaction cancelled at the administrator prompt")
		}
		return fmt.Errorf("start elevated compaction: %w", err)
	}
	return nil
}
