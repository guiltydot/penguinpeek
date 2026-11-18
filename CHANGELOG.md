# Changelog

All notable changes to PenguinPeek are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- The GitHub Copilot CLI shows up in Services while `copilot` or its
  `copilotd` daemon runs. It is shown only; PenguinPeek does not start or
  stop it.

## [1.0.1]

### Security
- Disk compaction no longer writes its elevated PowerShell script or its
  diskpart commands to fixed files in `%TEMP%`, where another program running
  as you could swap them before the administrator prompt. The script is
  passed inline, and PowerShell, `wsl.exe` and `diskpart.exe` are started by
  their full system paths.
- *Open home folder* and *Open in VS Code* fall back to the distro root when
  Linux reports an unusual `$HOME` (for example one containing `..`), so
  Explorer can never be pointed at another network share.

## [1.0.0]

First public release.

### Added
- Tray icon reflecting the selected distro's state, with a quick-action menu.
- Compact flyout: state, uptime, CPU with a one-minute sparkline, memory and
  swap, Start / Stop / Restart / Terminal, and an icon-aware distro picker.
- WSL VM memory as Windows sees it, and a **Free** button that releases
  Linux's file cache.
- Disk panel: space used inside Linux vs. the VHDX file, with guided trim and
  compaction.
- Services tab: status of Docker, PostgreSQL, Redis, MySQL/MariaDB, MongoDB
  and Nginx, with start and stop via systemd or `service`. A service that
  only runs inside a Docker container (such as the nginx bundled in a web app
  image) is shown with its container's name instead of a stop button.
- Containers tab: Docker containers grouped by Compose project; start, stop
  or restart a single container or a whole project. Works with a native
  dockerd and with Docker Desktop's WSL integration.
- Ports tab listing what listens inside Linux; open `localhost:<port>` in the
  browser or copy the address for databases.
- Windows notifications when a distro stops unexpectedly, a watched service
  stops, or WSL memory passes 90%, with a test button.
- Quick opens: home folder in File Explorer and VS Code (Remote - WSL).
- Start with Windows; light, dark or system theme; refresh every 2, 5 or 10 s.
- Lean by design: never boots a stopped distro, one `wsl.exe` call per
  refresh built from shell built-ins, slower checks while the flyout is
  hidden, and the WebView is released 60 s after it closes. WebView2 data
  lives in `%LOCALAPPDATA%\PenguinPeek\WebView2`. WSL2 only.
