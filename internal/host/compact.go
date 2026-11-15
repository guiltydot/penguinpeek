package host

import (
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// compactBody runs elevated after $Vhdx and $Sys are set. It shuts WSL down
// (diskpart cannot compact an attached disk), then compacts the VHDX
// read-only attached. Tools are called by full path and diskpart reads its
// commands from a pipe, so nothing on disk or on PATH can be swapped in.
const compactBody = `$ErrorActionPreference = 'Stop'
$Host.UI.RawUI.WindowTitle = 'PenguinPeek - compact WSL disk'
Write-Host "Compacting $Vhdx"
$before = (Get-Item -LiteralPath $Vhdx).Length
Write-Host 'Shutting down WSL...'
& (Join-Path $Sys 'wsl.exe') --shutdown
Start-Sleep -Seconds 3
@(
  "select vdisk file=""$Vhdx""",
  'attach vdisk readonly',
  'compact vdisk',
  'detach vdisk'
) | & (Join-Path $Sys 'diskpart.exe')
$after = (Get-Item -LiteralPath $Vhdx).Length
Write-Host ('Done: {0:N2} GB -> {1:N2} GB' -f ($before/1GB), ($after/1GB))
Read-Host 'Press Enter to close'
`

// compactCommand returns the elevated compaction script for vhdx as a
// PowerShell -EncodedCommand value (base64 of UTF-16LE). sysDir is the
// Windows system directory, e.g. C:\Windows\System32.
func compactCommand(vhdx, sysDir string) (string, error) {
	if !filepath.IsAbs(vhdx) && !strings.HasPrefix(vhdx, `\\`) && !(len(vhdx) > 2 && vhdx[1] == ':') {
		return "", errors.New("disk file path is not absolute")
	}
	if !strings.EqualFold(filepath.Ext(vhdx), ".vhdx") || strings.ContainsAny(vhdx, "\"\r\n\x00") ||
		strings.ContainsAny(sysDir, "'\"\r\n\x00") {
		return "", errors.New("unexpected disk file path")
	}
	script := "$Vhdx = " + psQuote(vhdx) + "\n$Sys = " + psQuote(sysDir) + "\n" + compactBody
	u := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// psQuote makes s a PowerShell single-quoted literal, in which nothing is
// expanded and a quote is written twice.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
