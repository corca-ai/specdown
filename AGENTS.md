# Agent Guide

`specdown` is a Markdown-first executable specification system.
The goal is to separate a reusable core from product-specific adapters.

## Read First

Read the following documents before starting work.

- [System design](docs/design.md) — scope, terminology, package boundaries, grammar, interfaces, and phased deliverables
- [Documentation guide](docs/metadoc.md) — rules for keeping docs short and accurate
- [Build & Run](docs/build.md) — how to build, run, and test

### User Guide

- [Writing Specs](docs/guide-spec.md) — how to write spec files
- [Writing Good Specs](docs/guide-writing.md) — best practices and Alloy/E2E patterns
- [Writing Adapters](docs/guide-adapter.md) — how to implement adapters and the protocol
- [Configuration & Running](docs/guide-config.md) — configuration files and execution
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
