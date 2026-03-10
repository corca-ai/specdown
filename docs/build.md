# Build & Run

## Prerequisites

A Go toolchain is required. The project uses [mise](https://mise.jdx.dev/) to manage it (`mise.toml`).

```sh
mise install        # install Go version from mise.toml
```

If `go` is not on `PATH` (common in non-interactive shells and CI agents),
resolve it via mise:

```sh
export PATH="$(mise where go)/bin:$PATH"
```

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

Run from the project root. Config lives next to the specs it governs.

```sh
specdown run -config specs/specdown.json
specdown version          # print build version
specdown alloy dump       # generate only Alloy model .als files
```

Reports are generated in `specs/report/`.

## Test

```sh
go test ./...
```

### Selfspecs

The project's own specifications are executable. After building both
binaries, run them from the project root. Selfspecs invoke `specdown`
recursively, so `bin/` must be on `PATH`:

```sh
PATH="$(pwd)/bin:$PATH" bin/specdown run -config specs/specdown.json
```

Reports are generated in `specs/report/`.

### Pocket-Board Example

```sh
cd examples/pocket-board && PATH="$(pwd)/../../bin:$PATH" ../../bin/specdown run
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
