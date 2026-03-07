# Overview

`specdown` is a Markdown-first executable specification system.
A single Markdown document serves as both a readable specification and an executable test.

Spec files are `*.spec.md` Markdown documents.
Prose is preserved as-is; only executable blocks, fixture tables, and Alloy model blocks are structurally interpreted.

A spec document can contain:

- **Executable blocks** — fenced code blocks prefixed with `run:`, `verify:`, `test:`, or `doctest:` that are dispatched to adapters for execution
- **Fixture tables** — Markdown tables preceded by a `<!-- fixture:name -->` directive, where each row becomes an independent test case
- **Alloy model blocks** — fenced code blocks with `alloy:model(name)` that embed formal verification fragments
- **Variables** — values captured from block output with `-> $name` and referenced with `${name}` in subsequent blocks and tables
- **Hooks** — `<!-- setup -->` and `<!-- teardown -->` directives that run adapter commands at section boundaries

After execution, specdown produces an HTML report that preserves the document structure and annotates each block and table row with pass/fail status.

## Three layers of specification

The power of specdown comes from weaving three complementary layers in a single document:

1. **Natural language** states design intent and rationale in prose. It explains *why* the system behaves a certain way, making the spec readable as a document — not just a test suite.

2. **Alloy models** prove structural properties exhaustively. A model can verify that "a card always belongs to exactly one board" holds for all possible states within a bounded scope — something no finite set of examples can guarantee.

3. **Executable blocks and fixture tables** confirm that the implementation matches. They test concrete behavior against the running system through adapters.

Each layer covers what the others cannot. Prose communicates intent to humans but cannot be executed. Alloy proves properties across all combinations but operates on an abstract model, not real code. Executable blocks test real code but can only cover the examples you write.

When all three live in the same section, the document tells a complete story: *what* the rule is (prose), *why* it holds universally (Alloy), and *that* the implementation obeys it (executable check). A failing Alloy check reveals a design flaw before any code runs. A failing executable block reveals an implementation bug even when the model is sound. And the prose ties both results back to the design decision that motivated them.
