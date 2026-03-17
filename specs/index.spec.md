# Specdown

A Markdown-first executable specification system.
One document is the spec, the test, and the report.

This page is itself a spec — it was executed by `specdown run` to produce the report you are reading. Separate docs and tests always drift apart: properties stated in documents go unverified, and tests confirm behavior but never explain design intent. specdown weaves natural language, executable acceptance tests, and optional [Alloy models](alloy.spec.md) into one Markdown file — prose explains intent, executable blocks confirm implementation, and formal models guarantee structural properties.

Inspired by Ward Cunningham's [FIT](https://en.wikipedia.org/wiki/Framework_for_integrated_test) and Donald Knuth's [Literate Programming](https://en.wikipedia.org/wiki/Literate_programming).

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

Green border = pass. Red border = failure. That's the whole idea: write prose, embed executable examples, get a verified report. See the [source Markdown](https://raw.githubusercontent.com/corca-ai/specdown/refs/heads/main/specs/index.spec.md) that produced this page.

## Chapters

### Fundamentals

- [Overview](overview.spec.md) — install and first spec
- [Spec Syntax](syntax.spec.md) — shell blocks, doctest blocks, variables, check tables, hooks
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
