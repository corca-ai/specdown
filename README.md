# specdown

Executable specifications in Markdown. One document is both a readable spec and a runnable test suite.

## Quick Start

```sh
specdown init        # scaffold a new project
specdown run         # execute specs and generate reports
```

## Install

### Binary (recommended)

```sh
curl -sSfL https://raw.githubusercontent.com/corca-ai/specdown/main/install.sh | sh
```

Installs to `/usr/local/bin`, or `~/.local/bin` if `/usr/local/bin` is not
writable. Ensure the install directory is on your `PATH`.

Or download directly from [Releases](https://github.com/corca-ai/specdown/releases/latest).

### go install

```sh
go install github.com/corca-ai/specdown/cmd/specdown@latest
```

### Homebrew

```sh
brew install corca-ai/tap/specdown
```

### From source

```sh
go build -o bin/specdown ./cmd/specdown
go build -o bin/specdown-adapter-shell ./cmd/specdown-adapter-shell
```

### Verify installation

```sh
specdown version
```

## Documentation

- [Overview](specs/overview.spec.md) — install, first spec, and why specdown exists
- [Self-Spec](specs/index.spec.md) — the executable reference
- [Live Report](https://corca-ai.github.io/specdown/) — self-spec execution results
- [Best Practices](specs/best-practices.spec.md) — patterns, pitfalls, and anti-patterns
- [Build & Run](docs/build.md) — building from source
- [Agent Guide](AGENTS.md) — project layout, working rules, and conventions

## Example

See [examples/pocket-board/](examples/pocket-board/) for a working project
that uses shell blocks and Alloy models without any custom adapters.

## License

[MIT](LICENSE)
