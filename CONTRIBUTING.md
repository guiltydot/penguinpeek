# Contributing to PenguinPeek

Thanks for helping! Bug reports, ideas and pull requests are all welcome.

## Ground rules

PenguinPeek's main promise is being **light**. Changes should keep it that way:

- **No polling in the frontend.** Go owns the single monitoring loop and pushes
  `snapshot` events; the UI only renders them.
- **Never boot a stopped distro** as a side effect. Anything that runs inside
  Linux must check the distro is already running, unless the user clicked an
  action that obviously starts it (Start, Terminal, Files, VS Code).
- **Stay quiet while hidden.** The hidden cadence is one `wsl.exe` call every
  30 s (plus one guest sample when alerts need it).
- **Root only on request.** Commands run as root only after an explicit click,
  and only fixed scripts are sent to `wsl.exe`; the frontend can pass catalog
  IDs and distro names, never shell text.
- It's a tray app. There is intentionally no dashboard window.

## Development setup

You need Windows 10/11 with WSL2, Go 1.27+, Node 22+, pnpm and the Wails CLI:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0   # inside WSL: CGO_ENABLED=0 go install ...
cd frontend && pnpm install && cd ..
wails3 build              # -> bin/PenguinPeek.exe (Windows; cross-compiles from WSL)
wails3 task install       # copy to %LOCALAPPDATA%\Programs\PenguinPeek, desktop shortcut, start
```

| Task | Command |
| --- | --- |
| Go tests | `go test -race ./internal/...` |
| Vet (Windows target) | `GOOS=windows go vet ./...` |
| Frontend type check | `cd frontend && pnpm check` |
| UI in a browser with mock data | `cd frontend && pnpm dev` |
| Regenerate TS bindings | `GOOS=windows wails3 generate bindings -clean=true -ts -i` |
| Probe your real WSL | `PEEK_LIVE=1 go test ./internal/wsl -run Live -v` |
| Benchmark the installed app | `wails3 task bench NOTE="what changed"` |

`frontend/bindings/` is generated; don't edit it by hand.

The mock UI accepts preview parameters, e.g.
`http://localhost:9245/?frame&theme=dark&tab=ports`, `?view=settings`,
`?view=disk`, `?gallery&picker`.

## Performance check

Run `wails3 task bench` before and after a change that could affect resource
use. It measures CPU and RAM while idle, with the flyout open, and after the
WebView is released, saves the run under `benchmarks/results/` (git-ignored)
and flags regressions against the previous run. Mention the numbers in your PR.

## Commits and versions

- Small, focused commits; [Conventional Commits](https://www.conventionalcommits.org/)
  style is appreciated (`feat:`, `fix:`, `perf:`, `docs:` ...).
- Versions follow [SemVer](https://semver.org/). `VERSION` is the single source
  of truth; bump it with `scripts/version.sh patch|minor|major` and add a
  `CHANGELOG.md` entry. Pushing a `vX.Y.Z` tag builds the release.
