# Build & Run

## Prerequisites

A Go toolchain is required. `go.mod` is the supported-version source of truth,
and the tracked `mise.toml` pins the same exact version for local development.
CI, self-spec, and release workflows use that version too.

```sh
mise install        # install Go version from mise.toml
```

If `go` is not on `PATH` (common in non-interactive shells and CI agents),
resolve it via mise:

```sh
export PATH="$(mise where go)/bin:$PATH"
```

When upgrading Go, update the `go` directive in `go.mod`, `mise.toml`, and all
three `actions/setup-go` entries together. Then run the drift check and normal
project gates:

```sh
scripts/check-go-version.sh
go test ./...
golangci-lint run
```

Do not use floating versions such as `latest`; toolchain upgrades must be
reviewed changes.

## Build

```sh
go build -o bin/specdown ./cmd/specdown
```

For release builds, inject the version via `ldflags`.

```sh
go build -trimpath -ldflags="-s -w -X main.version=v0.7.0" -o bin/specdown ./cmd/specdown
```

## Run

Run from the project root. Config lives at the project root (`specdown.json`).

```sh
specdown run
specdown version          # print build version
specdown alloy dump       # generate only Alloy model .als files
```

Reports are generated in `specs/report/`.

## Test

```sh
go test ./...
```

### Selfspecs

The project's own specifications are executable. After building,
run them from the project root. Selfspecs invoke `specdown`
recursively, so `bin/` must be on `PATH`:

```sh
PATH="$(pwd)/bin:$PATH" bin/specdown run
```

Reports are generated in `specs/report/`.

### Pocket-Board Example

```sh
cd examples/pocket-board && PATH="$(pwd)/../../bin:$PATH" ../../bin/specdown run
```

### Pre-commit hook

The repository includes a pre-commit hook in `.githooks/`. Enable it once after cloning:

```sh
git config core.hooksPath .githooks
```

The hook runs tests, lint, build, selfspecs, and example specs.

## Lint

The project uses [golangci-lint](https://golangci-lint.run/) with the configuration in `.golangci.yml`.

```sh
golangci-lint run
```

Enabled linters: errcheck, govet, staticcheck, unused, ineffassign, gocritic, gocognit, bodyclose, nilerr, errorlint, unparam, unconvert.

## CI

GitHub Actions runs on every push to `main` and on pull requests (`.github/workflows/ci.yml`).

The workflow pins Go 1.26.5 and `govulncheck` v1.6.0 so security results are
reproducible.

1. Verify and tidy Go modules.
2. Run `go test -race ./...`.
3. Run the blocking vulnerability scan.
4. Run `golangci-lint`.

## Release

Pushing a `v*` tag triggers [GoReleaser](https://goreleaser.com/) via GitHub Actions.
It cross-compiles for macOS, Linux, and Windows, creates archives with checksums,
and publishes a GitHub Release.

```sh
git tag v0.8.0
git push origin v0.8.0
```

Configuration is in `.goreleaser.yaml`. Keep the Homebrew `brews` entry pointed
at `directory: Formula`; the shared `corca-ai/homebrew-tap` uses `Formula/` as
the canonical formula directory, and root-level formula files are ignored once
that directory exists.
