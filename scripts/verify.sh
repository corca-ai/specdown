#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

task="${1:-}"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

verify_fast() (
  require_command go
  require_command golangci-lint

  echo "Checking toolchain and modules..."
  ./scripts/check-go-version.sh
  go mod verify
  go mod tidy -diff

  echo "Running race tests..."
  CGO_ENABLED=1 go test -race ./...

  echo "Running lint..."
  golangci-lint run ./...

  echo "Building specdown..."
  temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/specdown-verify-fast.XXXXXX")"
  trap 'rm -rf "$temp_dir"' EXIT
  go build -o "$temp_dir/specdown" ./cmd/specdown
)

verify_spec() (
  require_command go
  require_command java

  temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/specdown-verify-spec.XXXXXX")"
  trap 'rm -rf "$temp_dir"' EXIT

  echo "Building specdown for executable specifications..."
  go build -o "$temp_dir/specdown" ./cmd/specdown

  echo "Running self-specs..."
  PATH="$temp_dir:$PATH" "$temp_dir/specdown" run

  echo "Running pocket-board example specs..."
  (
    cd examples/pocket-board
    PATH="$temp_dir:$PATH" "$temp_dir/specdown" run
  )
)

verify_release() {
  require_command go
  require_command goreleaser

  echo "Checking GoReleaser configuration..."
  goreleaser check

  echo "Building a non-publishing release snapshot..."
  goreleaser release --snapshot --clean

  echo "Running installer smoke tests..."
  SPECDOWN_RELEASE_SNAPSHOT_DIR=.artifacts/goreleaser \
    go test -count=1 -run '^TestInstaller' .
}

verify_security() {
  require_command govulncheck

  echo "Scanning dependencies for known vulnerabilities..."
  govulncheck ./...
}

case "$task" in
  fast)
    verify_fast
    ;;
  spec)
    verify_spec
    ;;
  release)
    verify_release
    ;;
  security)
    verify_security
    ;;
  *)
    echo "usage: $0 {fast|spec|release|security}" >&2
    exit 2
    ;;
esac
