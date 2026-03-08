# Overview

`specdown` is a Markdown-first executable specification system.
A single Markdown document serves as both a readable specification and an executable test.

## Problem

When design documents and test code are separated, the following problems recur.

- It is difficult to track whether properties stated in design documents are actually verified
- Tests verify behavior but do not sufficiently explain design intent and rationale
- Security properties and state-space properties are hard to cover with example-based tests alone
- As documents, tests, and formal models evolve separately, consistency depends on human memory

## Goals

1. A single Markdown document serves as both a readable specification and an executable test.
2. Alloy models are woven into the same document in literate style, connecting to formal verification.
3. Table-based specifications (FIT style) are provided as a first-class feature.
4. Execution results are rendered as HTML, turning the document itself into an execution report with green/red status.

## Non-Goals

The following are excluded from the v1 scope.

- Replacing all tests with a Markdown DSL
- Automatically proving that the implementation is fully equivalent to the formal model
- Embedding Playwright, Vitest, Jest, or Bun's test runner into the core
- Including project-specific DSLs such as DOM selectors, shell transcripts, or editor actions in the core

## Three Layers of Specification

The power of specdown comes from weaving three complementary layers in a single document:

1. **Natural language** states design intent and rationale in prose. It explains *why* the system behaves a certain way, making the spec readable as a document — not just a test suite.

2. **Alloy models** prove structural properties exhaustively. A model can verify that "a card always belongs to exactly one board" holds for all possible states within a bounded scope — something no finite set of examples can guarantee.

3. **Executable blocks and fixture tables** confirm that the implementation matches. They test concrete behavior against the running system through adapters.

Each layer covers what the others cannot. Prose communicates intent to humans but cannot be executed. Alloy proves properties across all combinations but operates on an abstract model, not real code. Executable blocks test real code but can only cover the examples you write.

Co-locating explanation and test makes inconsistency visible. When the prose says "a newly created board should exist" and the executable block right below it fails, the contradiction is impossible to miss. Alloy fragments woven into the prose with `alloy:model(name)` blocks mean the reader sees the formal property next to the English explanation of why it matters.

What Alloy checks is the logical consistency of the model, not the correctness of the implementation. The value is that the model catches contradictions in the spec itself — before any code is written. A project can use executable blocks and fixture tables without any formal model.

## Document Format

Spec files are `*.spec.md` Markdown documents.
Prose is preserved as-is; only executable blocks, fixture tables, and Alloy model blocks are structurally interpreted.

A spec document can contain:

- **Executable blocks** — fenced code blocks prefixed with `run:`, `verify:`, `test:`, or `doctest:` that are dispatched to adapters for execution
- **Fixture tables** — Markdown tables preceded by a `> fixture:name` directive, where each row becomes an independent test case
- **Alloy model blocks** — fenced code blocks with `alloy:model(name)` that embed formal verification fragments
- **Variables** — values captured from block output with `-> $name` and referenced with `${name}` in subsequent blocks and tables
- **Hooks** — `> setup` and `> teardown` directives that run adapter commands at section boundaries
