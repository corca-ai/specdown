# specdown

Executable specifications in Markdown. One document is both a readable spec and a runnable test suite.

```
specdown run
```

## Why

Specs drift from tests. Tests lack context. Formal models live in separate files nobody reads. specdown puts all three in one Markdown document so they stay in sync.

See [Introduction](docs/introduction.md) for the full rationale.

## Install

Download a binary from [Releases](../../releases), or build from source:

```sh
go build -o specdown ./cmd/specdown
```

## Quick start

1. Create `specdown.json`:

```json
{
  "include": ["specs/**/*.spec.md"],
  "adapters": [
    {
      "name": "myapp",
      "command": ["python3", "./tools/myapp_adapter.py"],
      "protocol": "specdown-adapter/v1"
    }
  ]
}
```

2. Write a spec in `specs/example.spec.md` (see [Writing Specs](docs/guide-spec.md)).

3. Run:

```sh
specdown run
```

The HTML report is written to `.artifacts/specdown/report.html`.

## Documentation

- [Introduction](docs/introduction.md) — problem, approach, and how the three layers work together
- [Writing Specs](docs/guide-spec.md) — spec file syntax
- [Writing Adapters](docs/guide-adapter.md) — adapter protocol and examples
- [Configuration & Running](docs/guide-config.md) — `specdown.json` and CLI options
- [Build & Run](docs/build.md) — building from source and releasing
- [System Design](docs/design.md) — architecture and internals

## License

[MIT](LICENSE)
