---
type: spec
workdir: .tmp-test
---

# Internals

This chapter describes how specdown is built. It is not required for
writing specs, but helps adapter authors and contributors understand
the core/adapter/reporter separation.

You can verify the tool is available and see its version:

```run:shell
$ specdown version
dev
```

## Design Pillars

- **Readable and writable by every Markdown editor** — Spec files are plain Markdown with standard fenced blocks and blockquote directives. Any editor that supports frontmatter works without plugins.
- **Understandable by all stakeholders** — Prose, tables, and results are readable by designers, PMs, and QA — not just engineers. The document is the spec, not a wrapper around code.
- **Adapters are ordinary processes** — Any language works. An adapter is just an executable that reads and writes NDJSON on stdin/stdout. No SDK, no plugin API, no runtime coupling.
- **Core knows nothing about products** — The core parses Markdown and routes cases. It never imports test frameworks, knows filesystem layouts, or interprets block semantics. All domain logic lives in adapters.

## Architecture

Four components process a spec document:

- **Core** — parses Markdown (headings, prose, blocks, tables), computes variable scopes, assigns executable unit IDs, and extracts embedded Alloy model fragments. Produces an execution plan — a list of blocks and table rows tagged with adapter names. Never executes anything itself.
- **ModelRunner** — model verification flows through the same ordered case sequence as adapter results. Explicit Alloy references keep their Markdown ordinal; checks written only inside a model run after authored cases. In documents with hooks, each Alloy check is deferred until the applicable setup scope succeeds; hook-free documents may batch their checks.
- **Runtime Adapter** — receives each unit, runs the actual code, and emits pass/fail events.
- **Reporter** — collects events and renders the final HTML or JSON output.

All four components communicate through a common event schema. This means
a new reporter or a new adapter can be added without changing the core.

## Core and Adapter Boundary

Core parses [depends::spec documents](syntax.md) and produces an execution plan.
Adapters execute it via the [depends::adapter protocol](adapter-protocol.md).

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
| type     | string | `caseStarted`, `casePassed`, `caseFailed`, or `caseSkipped` |
| id       | SpecID | Unique identifier for the case                 |
| label    | string | Human-readable description of the case         |
| message  | string | Failure or skip diagnostic                     |
| expected | string | Expected value (failed events only)            |
| actual   | string | Actual value (failed events only)              |
| bindings | array  | Variable bindings captured during execution    |

Events flow from adapters into case results; model verification
results (via `ModelRunner`) are scheduled only after applicable setup hooks.
Setup and teardown executions use a separate lifecycle event with
`scope`, `phase`, `status`, location, duration, and optional failure message.
The reporter never sees raw adapter protocol messages — only the unified
events assembled by the engine.

```run:shell
# Verify setup failure prevents its Alloy check from producing artifacts
rm -rf internals-hook-order internals-hook-order-out
rm -f .artifacts/specdown/models/internals-hook-order-spec-md-order*.als
mkdir -p internals-hook-order
printf '# Hook order\n\n- [Spec](spec.md)\n' > internals-hook-order/index.md
shell_fence=$(printf '\140\140\140run:shell')
alloy_fence=$(printf '\140\140\140alloy:model(order)')
fence=$(printf '\140\140\140')
cat <<SPEC > internals-hook-order/spec.md
# Hook Order

> setup
$shell_fence
exit 7
$fence

$alloy_fence
module order
sig Item {}
assert exists { some Item }
$fence

> alloy:ref(order#exists, scope=3)
SPEC
printf '{"entry":"internals-hook-order/index.md","adapters":[],"reporters":[{"builtin":"json","outFile":"internals-hook-order-out/report.json"}]}' > internals-hook-order.json
if specdown run -config internals-hook-order.json -out internals-hook-order-out >/dev/null 2>&1; then
  exit 1
else
  test $? -eq 1
fi
```

```run:shell
$ jq -r '[.results[].cases[]?][0].status + ":" + [ .results[].lifecycleEvents[]? ][0].scope' internals-hook-order-out/report.json
skipped:section
$ test -z "$(find .artifacts/specdown/models -name 'internals-hook-order-spec-md-order*.als' -print -quit 2>/dev/null)" && echo no-alloy-artifacts
no-alloy-artifacts
```

## Reporter Contract

A reporter receives a `Report` value after execution completes and
writes output artifacts. The report contains:

- **Title** — derived from the entry document heading.
- **Results** — one `DocumentResult` per spec, each holding an ordered list of `CaseResult` values. Kind-specific fields are nested in `code`, `table`, or `alloy` sub-structs.
- **LifecycleEvents** — completed global setup/teardown executions; section hook events live on their `DocumentResult`.
- **Summary** — aggregate counts for specs, cases, and lifecycle executions.
- **TraceErrors** — validation messages from the traceability checker (if configured).
- **TraceGraph** — the document graph with typed edges (if configured).

Two built-in reporters are supported:

- **html** — writes a multi-page HTML site with a global table of contents, per-document pages, shared CSS/JS assets, and optional trace graph visualization.
- **json** — writes the full `Report` struct as indented JSON. The report includes a `schemaVersion` field (currently `3`).

Reporter selection is configured in [depends::specdown.json](config.md) via the `reporters` array. Each entry specifies a `builtin` name and an `outFile` path.

The JSON report is machine-readable and can be verified:

```run:shell
# Create a minimal project and run it with a JSON reporter
mkdir -p reporter-json/specs
printf '# T\n\n- [S](s.spec.md)\n' > reporter-json/specs/index.md
printf '# S\n\nProse.\n' > reporter-json/specs/s.spec.md
cat <<'CFG' > reporter-json/specdown.json
{"entry":"specs/index.md","adapters":[],"reporters":[{"builtin":"json","outFile":"out.json"}]}
CFG
specdown run -config reporter-json/specdown.json -quiet 2>&1 | tail -1
```

```run:shell
$ grep '"schemaVersion": 3' reporter-json/out.json
  "schemaVersion": 3,
```

## Parallel Execution

When `-jobs N` is greater than 1, the engine executes documents
with a bounded worker queue containing at most N workers. Each document gets
its own adapter sessions — sessions are never shared across documents.

Within a single document, cases execute sequentially in document order.
Variable bindings from earlier blocks are available to later blocks
within the same scope.

When `-max-failures` is reached, the shared run context is canceled. Documents
still waiting in the queue do not start, while in-flight adapter, shell, and
Alloy subprocesses are terminated. The completed result that reached the limit
is retained; canceled in-flight documents that did not complete are omitted
from the report.

```run:shell
# A failure cancels in-flight work and leaves queued documents unstarted
rm -f queue-running-marker queue-pending-marker
cat <<'ADAPTER' > queue-adapter.sh
#!/bin/sh
read -r request
case "$request" in
  *fail*) printf '{"id":1,"error":"failed"}\n' ;;
  *slow*) sleep 2; touch queue-running-marker; printf '{"id":1,"output":"late"}\n' ;;
  *pending*) touch queue-pending-marker; printf '{"id":1,"output":"started"}\n' ;;
esac
ADAPTER
chmod +x queue-adapter.sh
BT=$(printf '\140\140\140')
printf '%s\n' '# First' '' "\${BT}run:queue" 'fail' "\${BT}" > queue-first.md
printf '%s\n' '# Second' '' "\${BT}run:queue" 'slow' "\${BT}" > queue-second.md
printf '%s\n' '# Third' '' "\${BT}run:queue" 'pending' "\${BT}" > queue-third.md
printf '%s\n' '# Queue' '' '- [First](queue-first.md)' '- [Second](queue-second.md)' '- [Third](queue-third.md)' > queue-index.md
printf '%s\n' '{"entry":"queue-index.md","adapters":[{"name":"queue","command":["sh","./queue-adapter.sh"],"blocks":["run:queue"]}],"reporters":[{"builtin":"json","outFile":"queue-report.json"}]}' > queue-config.json
specdown run -config queue-config.json -jobs 2 -max-failures 1 -quiet >/dev/null 2>&1 || true
sleep 0.2
test ! -e queue-running-marker
test ! -e queue-pending-marker
grep -q '"relativeTo": "queue-first.md"' queue-report.json
! grep -q '"relativeTo": "queue-second.md"' queue-report.json
! grep -q '"relativeTo": "queue-third.md"' queue-report.json
```

The default is `-jobs 1` (sequential). Setting `-jobs` to the number
of CPU cores is safe because each goroutine blocks on adapter I/O,
not CPU.

Sequential execution is the default:

```run:shell
$ specdown run -dry-run 2>&1 | grep 'spec(s)'
...
```

## Alloy Runner Integration

For hook-free documents, the engine can batch all model verification before
the adapter case loop and index results by `SpecID`. For documents with hooks,
it schedules each Alloy case in the normal case sequence only after the
applicable setup scope succeeds. The `ModelRunner` interface keeps both paths
decoupled from the engine.

```go
ModelRunner
  RunDocument(ctx context.Context, plan DocumentPlan) -> []CaseResult
```

For each runner invocation:

1. Collects the applicable `CaseKindAlloy` cases from the plan.
2. Groups the cases by model name.
3. Bundles each model's embedded Alloy fragments into a `.als` file.
4. Invokes the Alloy solver (Java subprocess) on each bundle.
5. Maps solver output back to individual assertion results.

The runner caches the Alloy JAR under
`~/.cache/specdown/alloy/<version>/`. Every cached or newly downloaded JAR
must match the SHA-256 stored for that Alloy version. Automatic downloads
have a 30-second deadline and a 64 MiB response limit; a timeout, oversized
response, or checksum mismatch leaves no promoted JAR or partial temp file.

```run:shell
go test ../internal/specdown/alloy -run 'TestEnsureAlloyJar(DoesNotReuseUnversionedCache|RejectsChecksumMismatch|RejectsOversized|AppliesDownloadTimeout)' >/dev/null
echo alloy-download-verified
```

```run:shell
$ echo alloy-download-verified
alloy-download-verified
```

Alloy cases execute in document order within the normal case sequence.
Alloy failures respect `-max-failures` and stream progress inline.
