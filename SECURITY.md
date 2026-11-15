# Security policy

## Supported versions

Only the latest release receives fixes.

## Reporting a vulnerability

Please **don't open a public issue** for security problems. Use GitHub's
private vulnerability reporting instead (the repository's **Security** tab →
**Report a vulnerability**). You should get a reply within a week.

Areas that deserve particular care:

- Commands PenguinPeek runs inside WSL as `root` (service control, cache
  drop, `fstrim`) and the elevated disk compaction script.
- Anything that lets frontend input reach `wsl.exe` arguments.
- The per-user `Run` registry entry used for "Start with Windows".
