# Build & Run

## Prerequisites

A Go toolchain is required. Java 21 is required for the Alloy self-specs.
`go.mod` is the supported Go version source of truth, and the tracked
`mise.toml` pins Go, golangci-lint, and GoReleaser for local development.
CI, self-spec, and release workflows use the same versions.

```sh
mise install
go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
```

If `go` is not on `PATH` (common in non-interactive shells and CI agents),
resolve it via mise:

```sh
export PATH="$(mise where go)/bin:$PATH"
```

When upgrading Go, update the `go` directive in `go.mod`, `mise.toml`, and every
`actions/setup-go` entry together. The drift check enforces this contract.

## Verification

Checked-in verification entry points are the source of truth for local hooks
and CI:

```sh
./scripts/verify.sh fast
./scripts/verify.sh spec
./scripts/verify.sh coverage
./scripts/verify.sh release
./scripts/verify.sh security
```

| command | coverage | prerequisites |
| --- | --- | --- |
| `fast` | toolchain drift, module integrity/tidiness, race tests, lint, build | Go, golangci-lint |
| `spec` | full self-spec and pocket-board example | Go, Java 21 |
| `coverage` | internal-package profile and total/package regression policy | Go |
| `release` | GoReleaser config, non-publishing snapshot, installer smoke tests | Go, GoReleaser |
| `security` | pinned `govulncheck` scan | govulncheck |

All build outputs go to temporary or ignored directories. Module tidiness uses
`go mod tidy -diff`, so verification does not rewrite tracked module files.
Do not use floating tool versions such as `latest`; upgrades must be reviewed
changes.

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

## Test and executable specifications

The fast suite includes race-enabled unit tests. The project's own
specifications and the pocket-board example are executable through the shared
spec verification command:

```sh
./scripts/verify.sh fast
./scripts/verify.sh spec
```

### Coverage policy

`./scripts/verify.sh coverage` writes its profile to a temporary directory and
checks it against `scripts/coverage-policy.json`. The policy ratchets both the
internal-package total and each package with meaningful production behavior.
CLI, report, adapter protocol, engine, and trace packages have explicit floors
so a stable global percentage cannot hide a regression at a user-facing
boundary.

Packages without statements do not appear in Go coverage profiles and therefore
need no exception. Thin platform wrappers and test-support packages remain part
of the total but do not have individual floors; their behavior is exercised by
race, self-spec, and release smoke tests. Update a floor only with an explained
behavior or test change, never merely to make CI pass.

Reports are generated in `specs/report/`.

### Pre-commit hook

The repository includes a pre-commit hook in `.githooks/`. Enable it once after cloning:

```sh
git config core.hooksPath .githooks
```

The hook calls `verify.sh fast` and `verify.sh spec`, the same entry points used
by GitHub Actions.

## CI

GitHub Actions runs the full matrix below. Branch protection should require the
`test`, `selfspec`, `release-smoke`, and `dependency-review` jobs on pull
requests.

| workflow job | events | checked-in entry point or gate |
| --- | --- | --- |
| `CI / test` | pushes and pull requests | `verify.sh fast`, `verify.sh coverage`, then `verify.sh security` |
| `Self-Spec / selfspec` | pushes and pull requests | `verify.sh spec`; publishes Pages artifacts only from `main` |
| `CI / release-smoke` | pull requests | `verify.sh release`; never publishes |
| `CI / dependency-review` | pull requests | GitHub dependency review action |
| `Release / release` | `v*` tags | publishing GoReleaser workflow |

The workflow pins Go 1.26.5, golangci-lint 2.12.2, GoReleaser 2.17.1, and
`govulncheck` v1.6.0 so verification is reproducible. Dependency review is the
only GitHub-hosted gate without a local equivalent.

## Release

Pushing a `v*` tag triggers [GoReleaser](https://goreleaser.com/) via GitHub Actions.
It cross-compiles for macOS, Linux, and Windows, creates archives with checksums,
and publishes a GitHub Release.

```sh
git tag v0.8.0
git push origin v0.8.0
```

Configuration is in `.goreleaser.yaml`. The Homebrew binary package is
generated through `homebrew_casks` into the shared
`corca-ai/homebrew-tap` repository's `Casks/` directory.
