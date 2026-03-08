# specdown

Executable specifications in Markdown. One document is both a readable spec and a runnable test suite.

```
specdown run
```

## Install

### Binary (recommended)

```sh
curl -sSfL https://raw.githubusercontent.com/corca-ai/specdown/main/install.sh | sh
```

Or download directly from [Releases](https://github.com/corca-ai/specdown/releases/latest).

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
- [Live Report](https://corca-ai.github.io/specdown/) — self-spec execution results
- [Best Practices](selfspecs/best-practices.spec.md) — patterns, pitfalls, and anti-patterns
- [Build & Run](docs/build.md) — building from source
- [Agent Guide](AGENTS.md) — project layout, working rules, and conventions

## License

[MIT](LICENSE)
