# PenguinPeek resource benchmark. Measures the installed app the way it is
# used: idle in the tray, flyout open, and idle again after the WebView is
# released. Saves JSON per run and appends a row to benchmarks/HISTORY.md so
# later versions can be compared. Run through `wails3 task bench`.
#
# The flyout briefly appears on screen during the "open" phase.
param(
    [Parameter(Mandatory = $true)][string]$OutDir,
    [string]$Exe = (Join-Path $env:LOCALAPPDATA 'Programs\PenguinPeek\PenguinPeek.exe'),
    [int]$IdleSeconds = 60,
    [int]$OpenSeconds = 30,
    [int]$ReleaseWait = 65,
    [string]$Note = ''
)
$ErrorActionPreference = 'Stop'
# Dots, not locale commas, in every number we write.
[Threading.Thread]::CurrentThread.CurrentCulture = [Globalization.CultureInfo]::InvariantCulture

Add-Type @"
using System; using System.Runtime.InteropServices;
public class PeekBench {
  [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr h, uint m, IntPtr w, IntPtr l);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
}
"@

function Get-Tree([int]$RootId) {
    $all = Get-CimInstance Win32_Process -Property ProcessId, ParentProcessId, Name
    $ids = @($RootId); $frontier = @($RootId)
    while ($frontier.Count) {
        $next = @($all | Where-Object { $frontier -contains $_.ParentProcessId } | ForEach-Object { [int]$_.ProcessId })
        $ids += $next; $frontier = $next
    }
    Get-Process -Id $ids -ErrorAction SilentlyContinue
}

function Measure-Phase([string]$Name, [int]$RootId, [int]$Seconds) {
    $start = @{}
    foreach ($p in (Get-Tree $RootId)) { $start[$p.Id] = $p.TotalProcessorTime.TotalSeconds }
    $peakPriv = 0; $peakWs = 0
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds 5
        $t = Get-Tree $RootId
        $peakPriv = [math]::Max($peakPriv, ($t | Measure-Object PrivateMemorySize64 -Sum).Sum)
        $peakWs = [math]::Max($peakWs, ($t | Measure-Object WorkingSet64 -Sum).Sum)
    }
    $tree = Get-Tree $RootId
    $cpu = 0.0
    foreach ($p in $tree) {
        $base = if ($start.ContainsKey($p.Id)) { $start[$p.Id] } else { 0 }
        $cpu += $p.TotalProcessorTime.TotalSeconds - $base
    }
    $app = $tree | Where-Object Id -eq $RootId
    [ordered]@{
        phase            = $Name
        seconds          = $Seconds
        processes        = @($tree).Count
        cpuSeconds       = [math]::Round($cpu, 3)
        cpuPercentOfCore = [math]::Round($cpu / $Seconds * 100, 3)
        privateMB        = [math]::Round(($tree | Measure-Object PrivateMemorySize64 -Sum).Sum / 1MB, 1)
        workingSetMB     = [math]::Round(($tree | Measure-Object WorkingSet64 -Sum).Sum / 1MB, 1)
        peakPrivateMB    = [math]::Round($peakPriv / 1MB, 1)
        peakWorkingSetMB = [math]::Round($peakWs / 1MB, 1)
        appPrivateMB     = [math]::Round($app.PrivateMemorySize64 / 1MB, 1)
        appWorkingSetMB  = [math]::Round($app.WorkingSet64 / 1MB, 1)
    }
}

function Get-DirMB([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return 0 }
    [math]::Round(((Get-ChildItem -LiteralPath $Path -Recurse -File -Force -ErrorAction SilentlyContinue |
        Measure-Object Length -Sum).Sum) / 1MB, 1)
}

if (-not (Test-Path -LiteralPath $Exe)) { throw "PenguinPeek not installed at $Exe (run: wails3 task install)" }
$version = (Get-Item -LiteralPath $Exe).VersionInfo.ProductVersion
$exeName = Split-Path $Exe -Leaf

# --- disk ---
$disk = [ordered]@{
    exeMB          = [math]::Round((Get-Item -LiteralPath $Exe).Length / 1MB, 2)
    installDirMB   = Get-DirMB (Split-Path $Exe)
    webviewCacheMB = Get-DirMB (Join-Path $env:LOCALAPPDATA 'PenguinPeek\WebView2')
    settingsKB     = [math]::Round(((Get-ChildItem (Join-Path $env:APPDATA 'PenguinPeek') -File -ErrorAction SilentlyContinue |
        Measure-Object Length -Sum).Sum) / 1KB, 1)
}

$settingsPath = Join-Path $env:APPDATA 'PenguinPeek\settings.json'
$settings = if (Test-Path $settingsPath) { Get-Content $settingsPath -Raw | ConvertFrom-Json } else { $null }
$env:WSL_UTF8 = "1"
$running = @(& wsl.exe --list --running --quiet 2>$null | Where-Object { $_ -and $_.Trim() }).Count

# --- fresh start ---
# Started via Explorer so it runs as from the shortcut: launched directly from
# a packaged (MSIX) host terminal it would inherit that host's AppData
# virtualisation.
$procName = [IO.Path]::GetFileNameWithoutExtension($exeName)
Get-Process -Name $procName -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 2
Start-Process -FilePath explorer.exe -ArgumentList "`"$Exe`""
$proc = $null
for ($i = 0; $i -lt 40 -and -not $proc; $i++) {
    Start-Sleep -Milliseconds 250
    $proc = Get-Process -Name $procName -ErrorAction SilentlyContinue | Select-Object -First 1
}
if (-not $proc) { throw "PenguinPeek did not start" }
Write-Host "Started $exeName $version (pid $($proc.Id)); settling 15 s"
Start-Sleep -Seconds 15

$phases = @()
Write-Host "Phase 1/3: idle in tray, flyout never opened ($IdleSeconds s)"
$phases += Measure-Phase 'idle' $proc.Id $IdleSeconds

Write-Host "Phase 2/3: flyout open ($OpenSeconds s) - it will appear on screen"
Start-Process -FilePath explorer.exe -ArgumentList "`"$Exe`""   # second instance asks the first to show the flyout
Start-Sleep -Seconds 5
$phases += Measure-Phase 'open' $proc.Id $OpenSeconds

Write-Host "Phase 3/3: hidden, waiting $ReleaseWait s for the WebView release, then measuring 30 s"
$proc.Refresh()
[PeekBench]::PostMessage($proc.MainWindowHandle, 0x0006, [IntPtr]::Zero, [IntPtr]::Zero) | Out-Null # WM_ACTIVATE inactive -> hide
Start-Sleep -Seconds $ReleaseWait
$phases += Measure-Phase 'released' $proc.Id 30

$cpuInfo = Get-CimInstance Win32_Processor | Select-Object -First 1
$os = Get-CimInstance Win32_OperatingSystem
$result = [ordered]@{
    version   = $version
    timestamp = (Get-Date).ToString('s')
    note      = $Note
    machine   = [ordered]@{
        cpu     = $cpuInfo.Name.Trim()
        cores   = $cpuInfo.NumberOfLogicalProcessors
        ramGB   = [math]::Round($os.TotalVisibleMemorySize / 1MB, 1)
        windows = "$($os.Caption) build $($os.BuildNumber)"
    }
    conditions = [ordered]@{
        runningDistros  = $running
        refreshSeconds  = if ($settings) { $settings.refreshSeconds } else { 5 }
        alerts          = if ($settings -and $settings.alerts) { $settings.alerts } else { 'default (all on)' }
        watchedServices = if ($settings) { @($settings.watchedServices).Count } else { 3 }
    }
    disk   = $disk
    phases = $phases
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$file = Join-Path (Resolve-Path -LiteralPath $OutDir).Path "$version-$stamp.json"
$result | ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 -LiteralPath $file
Write-Host "Saved $file"

# --- compare with the previous run ---
$prev = Get-ChildItem -LiteralPath $OutDir -Filter *.json | Where-Object Name -ne (Split-Path $file -Leaf) |
    Sort-Object LastWriteTime | Select-Object -Last 1
$flags = @()
if ($prev) {
    $old = Get-Content $prev.FullName -Raw | ConvertFrom-Json
    Write-Host ""
    Write-Host "Compared with $($prev.Name):"
    foreach ($ph in $phases) {
        $o = $old.phases | Where-Object phase -eq $ph.phase
        if (-not $o) { continue }
        foreach ($k in 'cpuPercentOfCore', 'privateMB', 'workingSetMB') {
            $a = [double]$o.$k; $b = [double]$ph[$k]
            $delta = $b - $a
            # Ignore noise on tiny numbers: CPU below 0.2 % of a core, RAM below 5 MB.
            $floor = if ($k -eq 'cpuPercentOfCore') { 0.2 } else { 5 }
            $mark = ''
            if ($delta -gt $floor -and $a -gt 0 -and ($delta / $a) -gt 0.25) { $mark = '  <-- regression'; $flags += "$($ph.phase).$k" }
            Write-Host ("  {0,-9} {1,-17} {2,9} -> {3,9}{4}" -f $ph.phase, $k, $a, $b, $mark)
        }
    }
    foreach ($k in 'exeMB', 'installDirMB') {
        $a = [double]$old.disk.$k; $b = [double]$disk[$k]
        $mark = if ($a -gt 0 -and ($b - $a) / $a -gt 0.10) { $flags += "disk.$k"; '  <-- regression' } else { '' }
        Write-Host ("  disk      {0,-17} {1,9} -> {2,9}{3}" -f $k, $a, $b, $mark)
    }
}

# --- history table ---
$hist = Join-Path (Split-Path (Resolve-Path -LiteralPath $OutDir).Path) 'HISTORY.md'
if (-not (Test-Path -LiteralPath $hist)) {
    @(
        '# PenguinPeek benchmark history',
        '',
        'One row per `wails3 task bench` run. CPU is % of one logical core averaged over the phase;',
        'RAM is summed over PenguinPeek and its WebView2 helper processes. Idle = in the tray with the',
        'flyout never opened; Open = flyout visible with live metrics; Released = 60 s after hiding.',
        'Short-lived `wsl.exe` calls are not included in the CPU figures.',
        '',
        '| Date | Version | Exe MB | Idle CPU % | Idle private MB | Open CPU % | Open private MB | Released CPU % | Released private MB | Note |',
        '| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |'
    ) | Set-Content -Encoding UTF8 -LiteralPath $hist
}
$p = @{}; foreach ($ph in $phases) { $p[$ph.phase] = $ph }
$row = "| {0} | {1} | {2} | {3} | {4} | {5} | {6} | {7} | {8} | {9} |" -f (Get-Date -Format 'yyyy-MM-dd'), $version,
    $disk.exeMB, $p.idle.cpuPercentOfCore, $p.idle.privateMB, $p.open.cpuPercentOfCore, $p.open.privateMB,
    $p.released.cpuPercentOfCore, $p.released.privateMB, (($Note, ($flags -join ', ')) -ne '' -join '; ')
Add-Content -Encoding UTF8 -LiteralPath $hist -Value $row

Write-Host ""
$phases | ForEach-Object { [pscustomobject]$_ } | Format-Table phase, processes, cpuPercentOfCore, privateMB, workingSetMB, appPrivateMB, peakPrivateMB -AutoSize | Out-String | Write-Host
$disk.GetEnumerator() | ForEach-Object { Write-Host ("  {0,-15} {1}" -f $_.Key, $_.Value) }
if ($flags) { Write-Host ""; Write-Host "Regressions: $($flags -join ', ')"; exit 2 }
