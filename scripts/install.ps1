# Installs a built PenguinPeek.exe for the current user and puts a shortcut
# on the desktop. Run through `wails3 task install` (works from WSL too).
#
# The exe is copied to %LOCALAPPDATA%\Programs\PenguinPeek so the shortcut
# never points into the WSL filesystem (WebView2 dislikes \\wsl$ paths and
# the app could not start while WSL is stopped).
param(
    [Parameter(Mandatory = $true)][string]$Exe,
    [switch]$NoLaunch
)
$ErrorActionPreference = 'Stop'

$dest = Join-Path $env:LOCALAPPDATA 'Programs\PenguinPeek'
$target = Join-Path $dest 'PenguinPeek.exe'
New-Item -ItemType Directory -Force -Path $dest | Out-Null

# Only one instance runs; replace it so the new build takes over.
$running = Get-Process penguinpeek -ErrorAction SilentlyContinue
if ($running) {
    $running | Stop-Process -Force
    $running | Wait-Process -Timeout 10 -ErrorAction SilentlyContinue
}

Copy-Item -LiteralPath $Exe -Destination $target -Force
$version = (Get-Item $target).VersionInfo.ProductVersion

# GetFolderPath follows OneDrive desktop redirection.
$desktop = [Environment]::GetFolderPath('Desktop')
$lnk = Join-Path $desktop 'PenguinPeek.lnk'
$shell = New-Object -ComObject WScript.Shell
$sc = $shell.CreateShortcut($lnk)
$sc.TargetPath = $target
$sc.WorkingDirectory = $dest
$sc.IconLocation = "$target,0"
$sc.Description = 'PenguinPeek - WSL2 tray monitor'
$sc.Save()

"Installed PenguinPeek $version to $target"
"Shortcut: $lnk"

if (-not $NoLaunch) {
    # Launch through Explorer so PenguinPeek runs exactly as from the
    # shortcut. Started directly from a packaged (MSIX) host, such as an
    # editor or AI app's terminal, it would inherit that app's file-system
    # virtualisation and write its AppData into the host's package folder.
    Start-Process -FilePath explorer.exe -ArgumentList "`"$target`""
    'Started PenguinPeek (look for the penguin in the tray).'
}
