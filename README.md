# specdown

Markdown-first executable specifications. Write readable documents that double as runnable tests.

```
specdown run
```

A single Markdown file can contain:

- **Executable blocks** — code fences dispatched to adapters (`run:`, `verify:`)
- **Fixture tables** — FIT-style tabular specs with pass/fail per row
- **Alloy models** — literate Alloy fragments checked inline

Results are rendered into a self-contained HTML report.

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

- [Writing Specs](docs/guide-spec.md) — spec file syntax
- [Writing Adapters](docs/guide-adapter.md) — adapter protocol and examples
- [Configuration & Running](docs/guide-config.md) — `specdown.json` and CLI options
- [System Design](docs/design.md) — architecture and internals

## License

[MIT](LICENSE)
