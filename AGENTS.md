# Agent Guide

`specdown` is a Markdown-first executable specification system.

## Read First

- [Documentation guide](docs/metadoc.md) — rules for writing and maintaining docs
- [Build & Run](docs/build.md) — build commands, test, release workflow, and toolchain setup (`go` PATH resolution)

## Self-Specs (source of truth)

- [Overview](specs/overview.spec.md) — what specdown is and getting started
- [Spec Syntax](specs/syntax.spec.md) — shell blocks, doctest blocks, variables, check tables, hooks
- [Configuration](specs/config.spec.md) — specdown.json format
- [CLI](specs/cli.spec.md) — commands and flags
- [Adapter Protocol](specs/adapter-protocol.spec.md) — protocol reference and examples
- [Alloy Models](specs/alloy.spec.md) — embedding and verification
- [HTML Report](specs/report.spec.md) — report structure and failure diagnostics
- [Internals](specs/internals.spec.md) — architecture and core/adapter boundary
- [Best Practices](specs/best-practices.spec.md) — patterns, pitfalls, anti-patterns
- [Validation Rules](specs/validation.spec.md) — parse-time error checking
- [Traceability](specs/traceability.spec.md) — document traceability graph
- [Alloy Language Reference](docs/alloy-reference.md) — Alloy syntax and semantics

Note: `CLAUDE.md` is a symlink to `AGENTS.md`.
