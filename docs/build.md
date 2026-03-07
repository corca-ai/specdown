# Build & Run

## Prerequisites

A Go toolchain is required. The project uses nix + direnv to provide it automatically.

```sh
direnv allow   # first time only
```

After this, Go is available whenever you enter the project directory.

## Build

```sh
go build -o ~/.local/bin/specdown ./cmd/specdown
```

For release builds, inject the version via `ldflags`.

```sh
go build -trimpath -ldflags="-s -w -X main.version=v0.7.0" -o specdown ./cmd/specdown
```

## Run

Run from the project root. It reads `specdown.json` as the configuration.

```sh
specdown run
specdown version          # print build version
specdown alloy dump       # generate only Alloy model .als files
```

Reports are generated at `.artifacts/specdown/report.html`.

## Test

```sh
go test ./...
```

## Release

Pushing a `v*` tag triggers GitHub Actions to build Windows, macOS, and Linux binaries and attach them to the Release.

```sh
git tag v0.7.0
git push origin v0.7.0
```
