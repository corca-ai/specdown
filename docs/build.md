# Build & Run

## Prerequisites

A Go toolchain is required. The project uses nix + direnv to provide it automatically.

```sh
direnv allow   # first time only
```

After this, Go is available whenever you enter the project directory.

## Build

```sh
go build -o bin/specdown ./cmd/specdown
go build -o bin/specdown-adapter-shell ./cmd/specdown-adapter-shell
```

For release builds, inject the version via `ldflags`.

```sh
go build -trimpath -ldflags="-s -w -X main.version=v0.7.0" -o bin/specdown ./cmd/specdown
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

## Lint

The project uses [golangci-lint](https://golangci-lint.run/) with the configuration in `.golangci.yml`.

```sh
golangci-lint run
```

Enabled linters: errcheck, govet, staticcheck, unused, ineffassign, gocritic, gocognit, bodyclose, nilerr, errorlint, unparam, unconvert.

## CI

GitHub Actions runs on every push to `main` and on pull requests (`.github/workflows/ci.yml`).

1. `go test -race ./...`
2. `golangci-lint run`

## Release

Pushing a `v*` tag triggers [GoReleaser](https://goreleaser.com/) via GitHub Actions.
It cross-compiles for macOS, Linux, and Windows, creates archives with checksums,
and publishes a GitHub Release.

```sh
git tag v0.8.0
git push origin v0.8.0
```

Configuration is in `.goreleaser.yaml`.
