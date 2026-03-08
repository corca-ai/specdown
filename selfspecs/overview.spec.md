# Overview

`specdown` is a Markdown-first executable specification system.
A single Markdown document serves as both a readable specification and an executable test.

## Install

### Binary (recommended)

```sh
curl -sSfL https://raw.githubusercontent.com/corca-ai/specdown/main/install.sh | sh
```

### go install

```sh
go install github.com/corca-ai/specdown/cmd/specdown@latest
```

### Homebrew

```sh
brew install corca-ai/tap/specdown
```

## Getting Started

### Create a project

```sh
specdown init
```

This creates `specdown.json`, `specs/index.spec.md`, and `specs/example.spec.md`.

### Run specs

```sh
specdown run
```

Specs are parsed, executed via adapters, and results are rendered as an HTML report.
Use `-dry-run` to validate syntax without executing.

### Install Claude Code skill

```sh
specdown install skills
```

This installs the `/specdown` skill with syntax reference, adapter protocol, and best practices.

## What Goes in a Spec

Spec files are `*.spec.md` Markdown documents.
Prose is preserved as-is; only executable blocks, fixture tables, and Alloy model blocks are structurally interpreted.

- **Executable blocks** — fenced code blocks prefixed with `run:`, `verify:`, `test:`, or `doctest:` that are dispatched to adapters for execution
- **Fixture tables** — Markdown tables preceded by a `> fixture:name` directive, where each row becomes an independent test case
- **Alloy model blocks** — fenced code blocks with `alloy:model(name)` that embed formal verification fragments
- **Variables** — values captured from block output with `-> $name` and referenced with `${name}` in subsequent blocks and tables
- **Hooks** — `> setup` and `> teardown` directives that run adapter commands at section boundaries

## Why

When design documents and test code are separated:

- Properties stated in design documents may not be verified
- Tests verify behavior but do not explain design intent
- State-space properties are hard to cover with example-based tests alone
- Consistency between documents, tests, and models depends on human memory

specdown solves this by weaving three layers in a single document:

1. **Natural language** explains design intent and rationale
2. **Alloy models** prove structural properties exhaustively within a bounded scope
3. **Executable blocks and fixture tables** confirm the implementation matches

A project can use executable blocks and fixture tables without any formal model. Alloy is optional.
