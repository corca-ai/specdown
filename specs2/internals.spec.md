---
type: spec
---

# Internals

## Design Pillars

- **Readable and writable by every Markdown editor** — Spec files are plain Markdown with standard fenced blocks and blockquote directives. Any editor that supports frontmatter works without plugins.
- **Understandable by all stakeholders** — Prose, tables, and results are readable by designers, PMs, and QA — not just engineers. The document is the spec, not a wrapper around code.
- **Adapters are ordinary processes** — Any language works. An adapter is just an executable that reads and writes NDJSON on stdin/stdout. No SDK, no plugin API, no runtime coupling.
- **Core knows nothing about products** — The core parses [depends::spec documents](syntax.spec.md) and routes cases to adapters via the [depends::adapter protocol](adapter-protocol.spec.md). It never imports test frameworks, knows filesystem layouts, or interprets block semantics. All domain logic lives in adapters.

## Architecture

Two pipelines diverge from a single document.

```text
Spec Document (.spec.md)
    |
    +-- Core
    |     +-- heading / prose / block / table parsing
    |     +-- variable scope computation
    |     +-- executable unit ID assignment
    |     +-- embedded Alloy model extraction
    |
    +-- Runtime Adapter
    |     +-- test execution + event emission
    |
    +-- Reporter
    |     +-- HTML / JSON artifact generation
    |
    +-- Alloy Runner
          +-- model check + event emission
```

- Core structures the document and creates the execution plan
- Runtime adapter turns each block/table into an actual test or command
- Reporter turns execution events into human-readable output
- Alloy runner feeds formal verification results into the same event model
- Adapters connect as out-of-process commands

## Core and Adapter Boundary

Core is responsible for:

- Markdown parsing and heading hierarchy
- Extracting code blocks, directives, and tables
- Variable binding and scope computation
- `SpecID` generation
- Combining embedded Alloy fragments
- Generating a runtime-independent execution plan
- Defining the common event schema

Adapters are responsible for:

- Interpreting block semantics (`run:*`, doctest-style)
- Interpreting column semantics of check tables
- Connecting to external execution environments

Reporters are responsible for:

- Rendering execution results as HTML/JSON from the event stream

Core must not know about any specific test framework, product-specific filesystem layouts, product-specific command vocabularies, or the adapter implementation language.
