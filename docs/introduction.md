# Introduction

## The problem

Design documents and test code tend to diverge. Three things go wrong:

1. **Specs drift.** A prose document says one thing; the test asserts another. Nobody notices until a bug ships.
2. **Tests lack context.** A test proves a behavior but not why the behavior matters. The reasoning lives in a doc that may or may not be up to date.
3. **Formal models are invisible.** When a team writes an Alloy or TLA+ model, it lives in a separate file. It is checked independently, and the connection to the running system is maintained by convention.

## The approach

specdown merges three layers into a single Markdown file:

- **Prose** — natural-language explanation of intent, constraints, and rationale.
- **Executable blocks and fixture tables** — concrete scenarios that run against the real system via adapters.
- **Alloy models** — formal constraints checked exhaustively within a bounded scope.

Each layer covers a weakness of the others.

### Prose + executable specs

Co-locating explanation and test makes inconsistency visible. When the prose says "a newly created board should exist" and the executable block right below it fails, the contradiction is impossible to miss — both for a human reader and for an LLM reviewing the document.

### Executable specs + Alloy models

Example-based tests exercise specific scenarios. They cannot cover every combination. An Alloy model states a property ("every card belongs to exactly one column") and the solver checks all instances up to a given scope. If the property has a counterexample, the solver finds it.

What Alloy checks is the **logical consistency of the model**, not the correctness of the implementation. The model says `column: one Column`; the adapter enforces `card["column"] in ("todo", "doing", "done")`. These are maintained separately. The value is that the model catches contradictions in the spec itself — before any code is written.

### Prose + Alloy models

Alloy fragments are woven into the prose with `alloy:model(name)` blocks and `alloy:ref` directives. This means the reader sees the formal property next to the English explanation of why it matters. The HTML report shows pass/fail badges inline, so a counterexample appears in context rather than in a separate log file.

## What specdown is not

- It does not replace unit tests or integration tests. It targets the layer where design intent is communicated.
- It does not prove that the implementation matches the model. That bridge is maintained by the adapter and by human review.
- It does not require Alloy. A project can use executable blocks and fixture tables without any formal model.

## Further reading

- [Writing Specs](guide-spec.md) — syntax for blocks, tables, variables, and frontmatter
- [Writing Adapters](guide-adapter.md) — how to implement an adapter in any language
- [Configuration & Running](guide-config.md) — CLI flags and `specdown.json`
- [System Design](design.md) — architecture, protocol, and internals
