# Agent Guide

`specdown` is a Markdown-first executable specification system.
The goal is to separate a reusable core from product-specific adapters.

## Read First

Read the following documents before starting work.

- [System design](docs/design.md) — scope, terminology, package boundaries, grammar, interfaces, and phased deliverables
- [Documentation guide](docs/metadoc.md) — rules for keeping docs short and accurate
- [Build & Run](docs/build.md) — nix/direnv prerequisites, build commands, test, and release workflow

### User Guide

- [Specdown Self-Spec](selfspecs/specdown.spec.md) — executable reference for syntax, config, CLI, adapter protocol, and report behavior
- [Writing Good Specs](docs/guide-writing.md) — best practices and Alloy/E2E patterns
- [Adapter Tutorial](docs/guide-adapter-tutorial.md) — how to build an adapter from scratch
- [Alloy Language Reference](docs/alloy-reference.md) — syntax and semantics of the Alloy language

## Working Rules

- Treat `docs/design.md` as the source of truth for the current architecture.
- Maintain the responsibility boundary between `core` and `adapter`.
- Documentation and examples must follow current design terminology exactly.
  - executable block
  - fixture table
  - `alloy:model(name)`
  - HTML report
  - `SpecId`, `SpecEvent`
- Fix or remove outdated terminology and legacy architecture traces immediately upon discovery.

## Target Package Shape

The recommended package layout for the current design is as follows.

- `specdown-core`
- `specdown-cli`
- `specdown-adapter-protocol`
- `specdown-reporter-html`
- `specdown-alloy`
- `specdown-adapter-shell`
- `specdown-adapter-vitest`

## Documentation Notes

- `AGENTS.md` serves only as an entry point; detailed explanations go in separate documents.
- When adding new documents, keep them focused on small topics and link from the relevant parent document.
- Do not version control auto-generated documents.
- Examples must match the actual design and runtime behavior.

`CLAUDE.md` is maintained as a symlink to `AGENTS.md`.
