# Specdown

## Introduction

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

### Three layers of specification

The power of specdown comes from weaving three complementary layers in a single document:

1. **Natural language** states design intent and rationale in prose. It explains *why* the system behaves a certain way, making the spec readable as a document — not just a test suite.

2. **Alloy models** prove structural properties exhaustively. A model can verify that "a card always belongs to exactly one board" holds for all possible states within a bounded scope — something no finite set of examples can guarantee.

3. **Executable blocks and fixture tables** confirm that the implementation matches. They test concrete behavior against the running system through adapters.

Each layer covers what the others cannot. Prose communicates intent to humans but cannot be executed. Alloy proves properties across all combinations but operates on an abstract model, not real code. Executable blocks test real code but can only cover the examples you write.

When all three live in the same section, the document tells a complete story: *what* the rule is (prose), *why* it holds universally (Alloy), and *that* the implementation obeys it (executable check). A failing Alloy check reveals a design flaw before any code runs. A failing executable block reveals an implementation bug even when the model is sound. And the prose ties both results back to the design decision that motivated them.

## Getting Started

A specdown workflow has three parts: a spec document that describes behavior,
a configuration file that registers adapters, and the `specdown run` command.

### A Minimal Spec

A well-formed spec document needs only a heading and prose to parse successfully.
Executable blocks and fixture tables are added as needed.

```run:shell
mkdir -p .tmp-test
cat <<'SPEC' > .tmp-test/valid.spec.md
# Valid Spec

Some prose.

## Section

More prose.
SPEC
printf '# T\n\n- [Valid](valid.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/valid-cfg.json
{"entry":"index.spec.md","adapters":[]}
CFG
specdown run -config .tmp-test/valid-cfg.json -dry-run 2>&1
```

### Running Specdown

The CLI reports its version when invoked with `version`.

```run:shell -> $version
specdown version
```

```verify:shell
echo "${version}" | grep -qE '^[a-z0-9]'
```

Dry-run mode parses and validates spec files without executing adapters.
This is useful for checking syntax before a full run.

```run:shell -> $dryOutput
specdown run -config selfspec.json -dry-run 2>&1
```

The dry-run output lists discovered specs.

```verify:shell
echo "${dryOutput}" | grep -q "spec"
```

## Writing Specs

This section builds up spec syntax from simple to complex:
headings, executable blocks, variables, fixture tables, hooks, and frontmatter.

### Headings and Prose

Heading hierarchy (`#`, `##`, `###`, ...) is converted into a test suite hierarchy.
Prose paragraphs are preserved in the HTML report but are not execution targets.

### Executable Blocks

Executable blocks are fenced code blocks whose info string starts with a recognized prefix.

| Prefix | Meaning |
|--------|---------|
| `run:<target>` | Side-effecting executable block |
| `verify:<target>` | Assertion block |
| `test:<name>` | Named high-level test DSL |
| `doctest:<target>` | Inline command/output verification block |

The `<target>` is defined by the adapter, not the core.

The parser must recognize `run`, `verify`, `test`, and `doctest` as executable block kinds.

<!-- fixture:block-kind -->
| info | kind | target |
| --- | --- | --- |
| run:shell | run | shell |
| verify:api | verify | api |
| test:login | test | login |
| doctest:shell | doctest | shell |

### Variable Capture

A block can capture its output into a variable with `-> $varName`.

<!-- fixture:block-kind -->
| info | kind | target |
| --- | --- | --- |
| run:shell -> $id | run | shell |

Variables captured this way are referenced in subsequent blocks
and tables using `${variableName}`.

#### Scoping rules

- Variables from parent sections are readable in child sections
- Sibling sections at the same depth can share variables (in document order, only previously captured values)
- An unresolved variable is a compile-time error

#### Variable escaping

To output a literal `${...}`, escape it with a backslash: `\${literal}`.

```run:shell -> $escapeTest
printf 'ok'
```

```verify:shell
test "${escapeTest}" = "ok"
```

### Doctest Blocks

A `doctest:<target>` block pairs shell commands with their expected output
inline, similar to Python's doctest. Lines starting with `$ ` are commands;
subsequent lines until the next `$ ` or end of block are the expected output.

Commands are executed sequentially. On the first output mismatch, the block
fails with `expected` and `actual` values for diffing. Commands without
expected output lines are executed but only checked for exit status.

Doctest blocks do not support variable capture (`-> $name`).

```doctest:shell
$ echo hello
hello
$ echo one two three
one two three
```

A doctest block with no expected output still verifies the command succeeds.

```doctest:shell
$ true
```

Multi-line expected output is matched exactly.

```doctest:shell
$ printf 'line1\nline2\nline3'
line1
line2
line3
```

### Expected Failures

Any executable block can be marked with `!fail` to indicate that failure
is the expected outcome. The spec passes when the adapter reports failure,
and fails if the adapter unexpectedly succeeds.

This is useful for documenting error cases, showing what invalid input
looks like, or including negative examples in a spec without breaking CI.

`!fail` blocks do not support variable capture (`-> $name`).

#### Failing verify block

A command that exits non-zero is normally a failure. With `!fail`, it passes.

```verify:shell !fail
false
```

#### Failing doctest with output mismatch

This doctest intentionally shows the wrong expected output.
The `!fail` modifier makes the mismatch count as a pass.

```doctest:shell !fail
$ echo hello
goodbye
```

#### Failing run block

A run block that exits non-zero normally fails. With `!fail`, it passes.

```run:shell !fail
exit 1
```

#### Multi-line doctest mismatch

When multiple commands are present, the block fails fast on the first
mismatch. With `!fail`, that expected mismatch is a pass.

```doctest:shell !fail
$ echo first
first
$ echo second
WRONG
```

### Fixture Tables

A Markdown table becomes executable when preceded by a fixture directive.

The directive is an HTML comment of the form `<!-- fixture:name -->`.
The first row is the header. Each subsequent row is an independent test case.
Fixture names are defined by the adapter, not the core.

#### Cell escaping

Table cells support escape sequences that are processed by the core
before sending to the adapter.

| Sequence | Meaning |
|----------|---------|
| `\n` | newline |
| `\|` | literal pipe |
| `\\` | literal backslash |

Adapters always receive unescaped values.
The HTML report also unescapes cells, rendering `\n` as visible line breaks.

<!-- fixture:cell-escape -->
| input | expected |
| --- | --- |
| hello | hello |
| line1\nline2 | line1\nline2 |
| a\\\|b | a\\\|b |

#### Fixture parameters

Fixtures can accept parameters via `(key=value)` syntax.
Parameters are passed to the adapter as `fixtureParams` in the `runCase` message.
Multiple parameters are comma-separated: `<!-- fixture:name(key1=val1, key2=val2) -->`.

#### Parameterized fixture call

A fixture directive with parameters but no following table creates a single
assertion case. The adapter receives a `runCase` with `kind: "tableRow"`,
the fixture name, `fixtureParams` populated, and empty `columns`/`cells`.

This is useful for inline assertions that don't warrant a full table:

```markdown
<!-- fixture:check-user(field=plan, expected=STANDARD) -->
```

A fixture directive without parameters and without a table is a compile-time error.

### Setup and Teardown Hooks

Hooks run adapter commands at section boundaries.
A hook directive must be followed by an executable code block.

| Directive | Meaning |
|-----------|---------|
| `<!-- setup -->` | Run once before the first case in the heading subtree |
| `<!-- teardown -->` | Run once after the last case in the heading subtree |
| `<!-- setup:each -->` | Run before the first case of each immediate child section |
| `<!-- teardown:each -->` | Run after the last case of each immediate child section |

Hooks are not counted as test cases. Their results do not appear in the
case list, but a hook failure marks the document as failed.

A setup or teardown directive followed by an executable code block must parse successfully.

```run:shell
mkdir -p .tmp-test
printf '# Hook Test\n\n## Group\n\n<!-- setup:each -->\n```run:shell\necho init\n```\n\n### Scenario A\n\nSome prose.\n' > .tmp-test/hook-good.spec.md
printf '# T\n\n- [Hook](hook-good.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/hook-good-cfg.json
{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":["run:shell"]}]}
CFG
specdown run -config .tmp-test/hook-good-cfg.json -dry-run 2>&1
```

### Frontmatter

An optional YAML frontmatter can be placed at the top of a spec file.

| Key | Description |
|-----|-------------|
| `timeout` | Per-case execution time limit in milliseconds. 0 means unlimited |

If frontmatter is absent, defaults (unlimited) apply.

A spec with a timeout must still pass when the adapter responds quickly.

```run:shell
mkdir -p .tmp-test
cat <<'SPEC' > .tmp-test/timeout.spec.md
---
timeout: 5000
---

# Timeout Test

## Quick

A simple command that completes well within the timeout.
SPEC
printf '# T\n\n- [Timeout](timeout.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/timeout-cfg.json
{"entry":"index.spec.md","adapters":[]}
CFG
specdown run -config .tmp-test/timeout-cfg.json -dry-run 2>&1
```

## Configuration

specdown uses a data-only JSON configuration file.
The canonical config must not depend on any specific language runtime.

```json
{
  "entry": "specs/index.spec.md",
  "adapters": [
    {
      "name": "myapp",
      "command": ["python3", "./tools/adapter.py"],
      "blocks": ["run:myapp", "verify:myapp"],
      "fixtures": ["user-exists"]
    }
  ],
  "reporters": [
    { "builtin": "html", "outFile": ".artifacts/specdown/report.html" },
    { "builtin": "json", "outFile": ".artifacts/specdown/report.json" }
  ],
  "models": { "builtin": "alloy" }
}
```

### Entry File

The `entry` field points to a Markdown file whose H1 heading becomes the report title.
The entry file lists spec documents as Markdown links; their order determines the table of contents.

### Config Fields

| Field | Description |
|-------|-------------|
| `entry` | Path to the entry Markdown file. Its H1 is the report title; links define spec order |
| `adapters` | List of adapters that handle executable blocks and fixtures |
| `reporters` | Output generators. `html` and `json` builtins provided |
| `models` | Alloy model verification. Can be omitted if not used |

### Validation

A config file without `entry` must be rejected.

```verify:shell
mkdir -p .tmp-test
echo '{}' > .tmp-test/bad-config.json
! specdown run -config .tmp-test/bad-config.json 2>/dev/null
```

Two adapters with the same name must be rejected.

```verify:shell
mkdir -p .tmp-test
cat <<'CFG' > .tmp-test/dup-adapter.json
{
  "entry": "index.spec.md",
  "adapters": [
    {"name": "a", "command": ["true"], "blocks": ["run:x"]},
    {"name": "a", "command": ["true"], "blocks": ["run:y"]}
  ]
}
CFG
! specdown run -config .tmp-test/dup-adapter.json 2>/dev/null
```

## CLI Reference

### Commands

| Command | Description |
|---------|-------------|
| `specdown run` | Parse, execute, and generate reports in one pass |
| `specdown version` | Print the build version |
| `specdown alloy dump` | Generate Alloy model `.als` files without running adapters |

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `specdown.json` | Config file path |
| `-out` | (per config file) | HTML report output path |
| `-filter` | (none) | Heading path substring filter |
| `-jobs` | `1` | Number of spec files to run in parallel |
| `-dry-run` | `false` | Parse and validate only |

### Filter

The `-filter` flag runs only cases whose heading path contains the given string.

```run:shell -> $filterOutput
specdown run -config selfspec.json -dry-run -filter "Filter" 2>&1
```

```verify:shell
echo "${filterOutput}" | grep -q "Filter"
```

## Adapter Protocol

An adapter is an executable that exchanges NDJSON messages via stdin/stdout.
Any language works as long as it reads JSON from stdin and writes JSON to stdout.

### Protocol flow

1. specdown sends a `setup` message (no response required)
2. specdown sends `runCase` messages in document order, each with an integer `id`
3. The adapter responds to each case with `passed` or `failed`, echoing the `id`
4. specdown sends a `teardown` message (no response required)

### Request format

For executable blocks (`kind: "code"`):

```json
{
  "type": "runCase",
  "id": 1,
  "case": {
    "kind": "code",
    "block": "run:myapp",
    "source": "create-board",
    "captureNames": ["boardName"],
    "bindings": [{"name": "x", "value": "1"}]
  }
}
```

For fixture table rows (`kind: "tableRow"`):

```json
{
  "type": "runCase",
  "id": 2,
  "case": {
    "kind": "tableRow",
    "fixture": "board-exists",
    "fixtureParams": {"user": "alan"},
    "columns": ["board", "exists"],
    "cells": ["board-1", "yes"],
    "bindings": []
  }
}
```

Variables in `source` and `cells` are already substituted.
Cell escape sequences are already resolved.
The adapter can process values directly without additional substitution.

### Response format

```json
{"id": 1, "type": "passed"}
{"id": 1, "type": "passed", "bindings": [{"name": "boardName", "value": "board-1"}]}
{"id": 1, "type": "failed", "message": "expected 3, got 4"}
{"id": 1, "type": "failed", "message": "mismatch", "expected": "foo", "actual": "bar", "label": "row description"}
```

| Field | Description |
|-------|-------------|
| `id` | Correlation ID, must echo the request `id` |
| `type` | `"passed"` or `"failed"` |
| `message` | Error description (failed only) |
| `expected` | Expected value for structured diff (optional) |
| `actual` | Actual value for structured diff (optional) |
| `label` | Human-readable row identifier, overrides default (optional) |
| `bindings` | Captured variables to pass to subsequent cases (passed only) |

### Registration

Adapters declare their capabilities in `specdown.json`.
specdown routes each case to the adapter that declared the matching block or fixture.

```json
{
  "adapters": [{
    "name": "myapp",
    "command": ["python3", "./tools/adapter.py"],
    "blocks": ["run:myapp", "verify:myapp"],
    "fixtures": ["user-exists"]
  }]
}
```

### Adapter behavior

- A single adapter process handles multiple `runCase` requests during one spec run
- The adapter can maintain process-local state across requests
- A non-zero exit indicates infrastructure failure, not a case failure
- stderr is used for diagnostic output; only protocol messages go to stdout

## HTML Report and Artifacts

After a spec run, the HTML report is generated as a self-contained file.
It preserves the document structure, annotating execution results inline.

- Prose is displayed as-is; only execution results are annotated with status
- Status indicators appear at section, code block, table row, and alloy reference levels
- Pass is shown with green, fail with red
- Failed items display message, expected/actual diff inline
- A summary shows pass/fail counts

```run:shell
mkdir -p .tmp-test
cat <<'SPEC' > .tmp-test/report-test.spec.md
# Report Test

A minimal spec for testing report generation.
SPEC
printf '# Report Test\n\n- [Report](report-test.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/report-test.json
{"entry":"index.spec.md","adapters":[]}
CFG
specdown run -config .tmp-test/report-test.json -out report.html 2>&1 || true
```

The report file must exist and contain standard HTML structure.

```doctest:shell
$ test -f .tmp-test/report.html && echo yes
yes
$ grep -q '<html' .tmp-test/report.html && echo found
found
$ grep -q 'Report Test' .tmp-test/report.html && echo found
found
```

### Output files

| File | Description |
|------|-------------|
| `.artifacts/specdown/report.html` | Executed specification HTML report |
| `.artifacts/specdown/report.json` | Machine-readable results |
| `.artifacts/specdown/models/*.als` | Combined Alloy models |

## Failure Diagnostics

When an adapter returns `expected`/`actual` values on failure,
those values appear in both the CLI output and the JSON report.

The CLI output format for fixture table failures includes the row number
and optional label:

```
  FAIL  Heading > Path  [fixture-name] row 5 "description"
        error message
        expected: expected-value
        actual:   actual-value
```

```run:shell
mkdir -p .tmp-test
cat <<'ADAPTER' > .tmp-test/diag-adapter.sh
#!/bin/sh
# A minimal adapter that fails with expected/actual/label
while IFS= read -r line; do
  type=$(printf '%s' "$line" | grep -o '"type":"[^"]*"' | head -1 | cut -d'"' -f4)
  case "$type" in
    setup|teardown) ;;
    runCase)
      printf '%s\n' '{"id":1,"type":"failed","message":"content mismatch","expected":"alpha\\nbeta","actual":"alpha\\ngamma","label":"diag row"}'
      ;;
  esac
done
ADAPTER
chmod +x .tmp-test/diag-adapter.sh
cat <<'SPEC' > .tmp-test/diag.spec.md
# Diag Test

<!-- fixture:diag -->
| input | output |
| --- | --- |
| a | b |
SPEC
printf '# T\n\n- [Diag](diag.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/diag.json
{"entry":"index.spec.md","adapters":[{"name":"d","command":["sh","./diag-adapter.sh"],"blocks":[],"fixtures":["diag"]}],"reporters":[{"builtin":"html","outFile":"diag-report.html"}]}
CFG
specdown run -config .tmp-test/diag.json 2>&1 || true
```

The JSON report must contain the expected and actual fields, and the adapter label.

```doctest:shell
$ grep -q '"expected"' .tmp-test/report.json && echo found
found
$ grep -q '"actual"' .tmp-test/report.json && echo found
found
$ grep -q '"diag row"' .tmp-test/report.json && echo found
found
```

## Alloy Models

Alloy fragments can be embedded directly in a spec document using
`alloy:model(name)` code blocks.

Fragments with the same model name are combined in document order
into a single logical model. Only the first fragment may contain
a `module` declaration.

To link an assertion check result to the current section:

```
<!-- alloy:ref(modelName#assertionName, scope=5) -->
```

This directive displays the check result as a status badge in the HTML report.

### Formal Properties

The document model has a simple structural invariant:
every executable block belongs to exactly one heading scope.

```alloy:model(docmodel)
module docmodel

sig Heading {}

sig Block {
  scope: one Heading
}

sig TableRow {
  scope: one Heading
}
```

A block must not belong to more than one heading scope.

```alloy:model(docmodel)
assert blockBelongsToOneScope {
  all b: Block | one b.scope
}

check blockBelongsToOneScope for 5
```

<!-- alloy:ref(docmodel#blockBelongsToOneScope, scope=5) -->

A table row must not belong to more than one heading scope.

```alloy:model(docmodel)
assert rowBelongsToOneScope {
  all r: TableRow | one r.scope
}

check rowBelongsToOneScope for 5
```

<!-- alloy:ref(docmodel#rowBelongsToOneScope, scope=5) -->

## Validation Rules

specdown validates spec documents at parse time and rejects malformed input
before any adapter is invoked. The following errors are caught during parsing.

### Unclosed code block

A spec with an unclosed fenced code block must be rejected at parse time.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n```run:shell\necho hello\n' > .tmp-test/unclosed.spec.md
printf '# T\n\n- [Unclosed](unclosed.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/unclosed-cfg.json
{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":["run:shell"]}]}
CFG
! specdown run -config .tmp-test/unclosed-cfg.json 2>/dev/null
```

### Fixture without table

A fixture directive without parameters and not followed by a table must be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n<!-- fixture:x -->\n\nJust prose.\n' > .tmp-test/fnt.spec.md
printf '# T\n\n- [Fnt](fnt.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/fnt-cfg.json
{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":["run:shell"],"fixtures":["x"]}]}
CFG
! specdown run -config .tmp-test/fnt-cfg.json 2>/dev/null
```

A fixture directive with parameters but no table is valid (parameterized fixture call).

```run:shell
mkdir -p .tmp-test
cat <<'SPEC' > .tmp-test/fixture-call.spec.md
# Fixture Call

Some prose.
<!-- fixture:check(field=plan, expected=STANDARD) -->

More prose.
SPEC
printf '# T\n\n- [FC](fixture-call.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/fixture-call-cfg.json
{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":[],"fixtures":["check"]}]}
CFG
specdown run -config .tmp-test/fixture-call-cfg.json -dry-run 2>&1
```

### Hook without code block

A setup or teardown directive not followed by a code block must be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n<!-- setup:each -->\n\nJust prose.\n' > .tmp-test/hook-bad.spec.md
printf '# T\n\n- [Hook](hook-bad.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/hook-bad-cfg.json
{"entry":"index.spec.md","adapters":[]}
CFG
! specdown run -config .tmp-test/hook-bad-cfg.json 2>/dev/null
```

### Unresolved variable

Referencing a variable that was never captured must produce an error.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n```run:shell\necho \${missing}\n```\n' > .tmp-test/unresolved.spec.md
printf '# T\n\n- [Unresolved](unresolved.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/unresolved-cfg.json
{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":["run:shell"]}]}
CFG
specdown run -config .tmp-test/unresolved-cfg.json 2>&1 | grep -q "missing"
! specdown run -config .tmp-test/unresolved-cfg.json 2>/dev/null
```
