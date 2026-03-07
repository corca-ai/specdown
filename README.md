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
  "entry": "specs/index.spec.md",
  "adapters": [
    {
      "name": "myapp",
      "command": ["python3", "./tools/myapp_adapter.py"],
      "blocks": ["run:myapp", "verify:myapp"]
    }
  ]
}
```

2. Create an entry file at `specs/index.spec.md`:

```markdown
# My Project Spec

- [Example](example.spec.md)
```

3. Write a spec in `specs/example.spec.md` (see [Self-Spec](selfspecs/specdown.spec.md) for syntax reference).

4. Run:

```sh
specdown run
```

The HTML report is written to `.artifacts/specdown/report.html`.

## Documentation

- [Introduction](docs/introduction.md) — problem, approach, and how the three layers work together
- [Specdown Self-Spec](selfspecs/specdown.spec.md) — executable reference for syntax, config, CLI, adapter protocol, and report behavior
- [Adapter Tutorial](docs/guide-adapter-tutorial.md) — how to build an adapter from scratch
- [Writing Good Specs](docs/guide-writing.md) — best practices and Alloy/E2E patterns
- [Build & Run](docs/build.md) — building from source and releasing
- [System Design](docs/design.md) — architecture and internals

## License

[MIT](LICENSE)
