#!/usr/bin/env bash
# Semantic versioning for PenguinPeek. VERSION is the single source of truth;
# this script propagates it to the Wails build config (Windows file version,
# manifest, NSIS, MSIX) and frontend/package.json.
#
#   scripts/version.sh                 print the current version
#   scripts/version.sh patch|minor|major
#   scripts/version.sh set 1.2.0-beta.1
#
# It never commits or tags; it prints the git commands to run.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

semver_re='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'

current="$(tr -d '[:space:]' < VERSION)"
if [[ ! $current =~ $semver_re ]]; then
  echo "VERSION contains '$current', which is not a semantic version" >&2
  exit 1
fi
major="${BASH_REMATCH[1]}" minor="${BASH_REMATCH[2]}" patch="${BASH_REMATCH[3]}"

case "${1:-show}" in
  show) echo "$current"; exit 0 ;;
  major) next="$((major + 1)).0.0" ;;
  minor) next="$major.$((minor + 1)).0" ;;
  patch)
    # Bumping a pre-release (1.1.0-beta.1) releases it as 1.1.0.
    if [[ $current == *-* ]]; then next="$major.$minor.$patch"; else next="$major.$minor.$((patch + 1))"; fi
    ;;
  set)
    next="${2:-}"
    if [[ ! $next =~ $semver_re ]]; then
      echo "usage: $0 set X.Y.Z[-prerelease]" >&2
      exit 1
    fi
    ;;
  *) echo "usage: $0 [show|patch|minor|major|set X.Y.Z[-prerelease]]" >&2; exit 1 ;;
esac

[[ $next =~ $semver_re ]]
# Windows version resources only accept numbers, so they get the X.Y.Z core.
core="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}"

echo "$next" > VERSION
sd 'version: "[^"]*" # The application version' "version: \"$core\" # The application version" build/config.yml
tmp="$(mktemp)"
jq --arg v "$next" '.version = $v' frontend/package.json > "$tmp" && mv "$tmp" frontend/package.json

(cd build && wails3 update build-assets -name PenguinPeek -binaryname PenguinPeek -config config.yml -dir . < /dev/null > /dev/null)
# The updater recreates platform folders this Windows-only app doesn't ship.
rm -rf build/darwin build/ios build/linux

echo "$current -> $next"
echo "Release with:"
echo "  git commit -am \"chore(release): v$next\""
echo "  git tag -a v$next -m \"PenguinPeek v$next\""
