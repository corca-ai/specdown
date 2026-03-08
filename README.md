# specdown

Executable specifications in Markdown. One document is both a readable spec and a runnable test suite.

```
specdown run
```

## Install

### Binary (recommended)

Download from [Releases](https://github.com/corca-ai/specdown/releases/latest):

```sh
# macOS (Apple Silicon)
curl -sSfL https://github.com/corca-ai/specdown/releases/latest/download/specdown_*_darwin_arm64.tar.gz | tar xz -C /usr/local/bin specdown

# macOS (Intel)
curl -sSfL https://github.com/corca-ai/specdown/releases/latest/download/specdown_*_darwin_amd64.tar.gz | tar xz -C /usr/local/bin specdown

# Linux (amd64)
curl -sSfL https://github.com/corca-ai/specdown/releases/latest/download/specdown_*_linux_amd64.tar.gz | tar xz -C /usr/local/bin specdown
```

### go install

```sh
go install github.com/corca-ai/specdown/cmd/specdown@latest
```

### From source

```sh
go build -o specdown ./cmd/specdown
```

## Documentation

- [Self-Spec](selfspecs/index.spec.md) — the executable reference (start here)
- [Writing Good Specs](docs/guide-writing.md) — best practices
- [Build & Run](docs/build.md) — building from source
- [Agent Guide](AGENTS.md) — project layout, working rules, and conventions

## License

[MIT](LICENSE)
