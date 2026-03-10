---
type: spec
---

# Internals

This chapter describes how specdown is built. It is not required for
writing specs, but helps adapter authors and contributors understand
the core/adapter/reporter separation.

You can verify the tool is available and see its version:

```run:shell
$ specdown version
...
```

## Design Pillars

- **Readable and writable by every Markdown editor** — Spec files are plain Markdown with standard fenced blocks and blockquote directives. Any editor that supports frontmatter works without plugins.
- **Understandable by all stakeholders** — Prose, tables, and results are readable by designers, PMs, and QA — not just engineers. The document is the spec, not a wrapper around code.
- **Adapters are ordinary processes** — Any language works. An adapter is just an executable that reads and writes NDJSON on stdin/stdout. No SDK, no plugin API, no runtime coupling.
- **Core knows nothing about products** — The core parses Markdown and routes cases. It never imports test frameworks, knows filesystem layouts, or interprets block semantics. All domain logic lives in adapters.

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

The core parses the document and produces an execution plan — a list of
blocks and table rows tagged with adapter names. It never executes anything
itself. The runtime adapter receives each unit, runs the actual code, and
emits pass/fail events. The reporter collects those events and renders
the final HTML or JSON output. The Alloy runner is a parallel path:
it extracts embedded model fragments, invokes the Alloy solver, and
feeds results into the same event stream.

All four components communicate through a common event schema. This means
a new reporter or a new adapter can be added without changing the core.

## Core and Adapter Boundary

Core parses [depends::spec documents](syntax.spec.md) and produces an execution plan.
Adapters execute it via the [depends::adapter protocol](adapter-protocol.spec.md).

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

A dry run demonstrates the boundary: the core parses and validates
without launching any adapter.

```run:shell
$ specdown run -dry-run 2>&1 | grep 'spec(s)'
...
```
