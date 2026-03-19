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

Four components process a spec document:

- **Core** — parses Markdown (headings, prose, blocks, tables), computes variable scopes, assigns executable unit IDs, and extracts embedded Alloy model fragments. Produces an execution plan — a list of blocks and table rows tagged with adapter names. Never executes anything itself.
- **ModelRunner** — the engine calls `ModelRunner.RunDocument()` before the case loop. Model verification results are pre-indexed and looked up inline during case processing, flowing through the same case sequence as adapter results.
- **Runtime Adapter** — receives each unit, runs the actual code, and emits pass/fail events.
- **Reporter** — collects events and renders the final HTML or JSON output.

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

## Event Schema

All components communicate through a common event type. Each event
carries a type, a case identifier, and optional diagnostic fields:

| Field    | Type   | Description                                    |
| -------- | ------ | ---------------------------------------------- |
| type     | string | `caseStarted`, `casePassed`, or `caseFailed`   |
| id       | SpecID | Unique identifier for the case                 |
| label    | string | Human-readable description of the case         |
| message  | string | Failure reason (failed events only)            |
| expected | string | Expected value (failed events only)            |
| actual   | string | Actual value (failed events only)              |
| bindings | array  | Variable bindings captured during execution    |

Events flow from adapters into case results; model verification
results (via `ModelRunner`) are pre-computed and looked up inline.
The reporter never sees raw adapter protocol messages — only the
unified event stream assembled by the engine.

## Reporter Contract

A reporter receives a `Report` value after execution completes and
writes output artifacts. The report contains:

- **Title** — derived from the entry document heading.
- **Results** — one `DocumentResult` per spec, each holding an ordered list of `CaseResult` values. Kind-specific fields are nested in `code`, `table`, or `alloy` sub-structs.
- **Summary** — aggregate counts: specs total/passed/failed, cases total/passed/failed/expected-fail.
- **TraceErrors** — validation messages from the traceability checker (if configured).
- **TraceGraph** — the document graph with typed edges (if configured).

Two built-in reporters are supported:

- **html** — writes a multi-page HTML site with a global table of contents, per-document pages, shared CSS/JS assets, and optional trace graph visualization.
- **json** — writes the full `Report` struct as indented JSON. The report includes a `schemaVersion` field (currently `2`).

Reporter selection is configured in [depends::specdown.json](config.spec.md) via the `reporters` array. Each entry specifies a `builtin` name and an `outFile` path.

The JSON report is machine-readable and can be verified:

```run:shell
# Create a minimal project and run it with a JSON reporter
mkdir -p .tmp-test/reporter-json/specs
printf '# T\n\n- [S](s.spec.md)\n' > .tmp-test/reporter-json/specs/index.spec.md
printf '# S\n\nProse.\n' > .tmp-test/reporter-json/specs/s.spec.md
cat <<'CFG' > .tmp-test/reporter-json/specdown.json
{"entry":"specs/index.spec.md","adapters":[],"reporters":[{"builtin":"json","outFile":"out.json"}]}
CFG
specdown run -config .tmp-test/reporter-json/specdown.json -quiet 2>&1 | tail -1
```

```run:shell
$ cat .tmp-test/reporter-json/out.json | head -1
{
```

## Parallel Execution

When `-jobs N` is greater than 1, the engine executes documents
concurrently using a semaphore of size N. Each document gets its own
adapter sessions — sessions are never shared across documents.

Within a single document, cases execute sequentially in document order.
Variable bindings from earlier blocks are available to later blocks
within the same scope.

The default is `-jobs 1` (sequential). Setting `-jobs` to the number
of CPU cores is safe because each goroutine blocks on adapter I/O,
not CPU.

Sequential execution is the default:

```run:shell
$ specdown run -dry-run 2>&1 | grep 'spec(s)'
...
```

## Alloy Runner Integration

The engine calls `ModelRunner.RunDocument()` before the case loop.
Results are indexed by `SpecID` and looked up inline during case
processing. The `ModelRunner` interface keeps model verification
decoupled from the engine.

```text
ModelRunner
  RunDocument(plan) -> []CaseResult
```

For each document, the runner:

1. Collects all `CaseKindAlloy` cases from the plan.
2. Groups them by model name.
3. Bundles embedded Alloy fragments into a single `.als` file per model.
4. Invokes the Alloy solver (Java subprocess) on each bundle.
5. Maps solver output back to individual assertion results.

The runner caches the Alloy JAR in `~/.cache/specdown/`. If the JAR
is missing, it downloads the official release automatically.

Model results are pre-computed via `ModelRunner` before the case loop;
alloy cases execute in document order within the normal case sequence.
Alloy failures respect `-max-failures` and stream progress inline.
