# Specdown

A Markdown-first executable specification system.
One document is the spec, the test, and the report.

This page is itself a spec — it was executed by `specdown run` to produce the report you are reading. Separate docs and tests always drift apart. specdown weaves natural language and executable acceptance tests into one Markdown file, and adapters connect those tests to real code — so prose, verification, and implementation stay linked by construction.

## See It Work

A passing block renders with a green left border:

```run:shell
$ echo "specifications as code"
specifications as code
```

A block marked `!fail` expects failure — it renders red but does not break the build:

```run:shell !fail
$ echo actual
expected
```

Green border = pass. Red border = expected failure. That's the whole idea: write prose, embed executable examples, get a verified report. See the [source Markdown](https://raw.githubusercontent.com/corca-ai/specdown/refs/heads/main/specs/index.spec.md) that produced this page.

## Chapters

### Fundamentals

- [Overview](overview.spec.md) — install, first spec, and why specdown exists
- [Spec Syntax](syntax.spec.md) — executable blocks, variables, check tables, hooks
- [Configuration](config.spec.md) — `specdown.json` format and defaults
- [CLI](cli.spec.md) — commands, flags, and filtering

### Adapters and Models

- [Adapter Protocol](adapter-protocol.spec.md) — NDJSON process protocol for any language
- [Alloy Models](alloy.spec.md) — embedding and verifying formal models

### Correctness

- [Validation Rules](validation.spec.md) — parse-time error checking
- [Traceability](traceability.spec.md) — document traceability with typed edges

### Reporting and Internals

- [HTML Report](report.spec.md) — multi-page report structure and failure diagnostics
- [Internals](internals.spec.md) — architecture and core/adapter boundary
- [Best Practices](best-practices.spec.md) — patterns, pitfalls, and when to use Alloy
