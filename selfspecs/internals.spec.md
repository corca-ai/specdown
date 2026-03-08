# Internals

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

- Interpreting block semantics (`run:*`, `verify:*`, `test:*`, `doctest:*`)
- Interpreting column semantics of fixture tables
- Connecting to external execution environments

Reporters are responsible for:

- Rendering execution results as HTML/JSON from the event stream

Core must not know about any specific test framework, product-specific filesystem layouts, product-specific command vocabularies, or the adapter implementation language.
