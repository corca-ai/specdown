# Agent Guide

`specdown` is a Markdown-first executable specification system.
The goal is to separate a reusable core from product-specific adapters.

## Read First

Read the following documents before starting work.

- [Documentation guide](docs/metadoc.md) — rules for keeping docs short and accurate
- [Build & Run](docs/build.md) — nix/direnv prerequisites, build commands, test, and release workflow

### User Guide

- [Specdown Self-Spec](selfspecs/specdown.spec.md) — executable reference for syntax, config, CLI, adapter protocol, and report behavior
- [Best Practices](selfspecs/best-practices.spec.md) — patterns, pitfalls, and anti-patterns
- [Adapter Protocol](selfspecs/adapter-protocol.spec.md) — protocol reference and adapter examples
- [Alloy Language Reference](docs/alloy-reference.md) — syntax and semantics of the Alloy language

## Working Rules

- Treat the self-specs (`selfspecs/`) as the source of truth for behavior and architecture.
- Maintain the responsibility boundary between `core` and `adapter`.
- Documentation and examples must follow current design terminology exactly.
  - executable block
  - fixture table
  - `alloy:model(name)`
  - HTML report
  - `SpecID`, `Event`
- Fix or remove outdated terminology and legacy architecture traces immediately upon discovery.

## Package Layout

- `cmd/specdown/` — CLI entry point
- `cmd/specdown-adapter-shell/` — builtin shell adapter
- `internal/specdown/core/` — parser, AST, planning, event model
- `internal/specdown/adapterprotocol/` — adapter process contract
- `internal/specdown/adapterhost/` — adapter process launcher
- `internal/specdown/engine/` — orchestrates spec execution
- `internal/specdown/alloy/` — Alloy model extraction + runner
- `internal/specdown/config/` — specdown.json loader
- `internal/specdown/reporter/html/` — HTML report renderer
- `internal/specdown/reporter/json/` — JSON report renderer

## Documentation Notes

- `AGENTS.md` serves only as an entry point; detailed explanations go in separate documents.
- When adding new documents, keep them focused on small topics and link from the relevant parent document.
- Do not version control auto-generated documents.
- Examples must match the actual design and runtime behavior.

`CLAUDE.md` is maintained as a symlink to `AGENTS.md`.
