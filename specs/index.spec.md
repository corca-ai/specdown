# Specdown

A Markdown-first executable specification system.
One document is the spec, the test, and the report.

New to specdown? Read the chapters below in order — each builds on the previous one.

## Fundamentals

How to write and run specs.

- [Overview](overview.spec.md) — install, first spec, and why specdown exists
- [Spec Syntax](syntax.spec.md) — executable blocks, variables, check tables, hooks
- [Configuration](config.spec.md) — `specdown.json` format and defaults
- [CLI](cli.spec.md) — commands, flags, and filtering

## Adapters and Models

Connecting specs to code and formal properties.

- [Adapter Protocol](adapter-protocol.spec.md) — NDJSON process protocol for any language
- [Alloy Models](alloy.spec.md) — embedding and verifying formal models

## Correctness

Ensuring specs and documents are well-formed.

- [Validation Rules](validation.spec.md) — parse-time error checking
- [Trace Graph](trace.spec.md) — document traceability with typed edges

## Reporting and Internals

What specdown produces and how it works.

- [HTML Report](report.spec.md) — multi-page report structure and failure diagnostics
- [Internals](internals.spec.md) — architecture and core/adapter boundary
- [Best Practices](best-practices.spec.md) — patterns, pitfalls, and when to use Alloy
