# Specdown

`specdown` is a Markdown-first executable specification system.
A single Markdown document serves as both a readable specification and an executable test.

Spec files are `*.spec.md` Markdown documents.
Prose is preserved as-is; only executable blocks, fixture tables, and Alloy model blocks are structurally interpreted.

## CLI

### Version

The CLI reports its version when invoked with `version`.

```run:shell -> $version
specdown version
```

```verify:shell
echo "${version}" | grep -qE '^[a-z0-9]'
```

### Dry Run

Dry-run mode parses and validates spec files without executing adapters.
This is useful for checking syntax before a full run.

```run:shell -> $dryOutput
specdown run -config selfspec.json -dry-run 2>&1
```

The dry-run output lists discovered specs.

```verify:shell
echo "${dryOutput}" | grep -q "spec"
```

### Filter

The `-filter` flag runs only cases whose heading path contains the given string.

```run:shell -> $filterOutput
specdown run -config selfspec.json -dry-run -filter "Version" 2>&1
```

```verify:shell
echo "${filterOutput}" | grep -q "Version"
```

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `specdown.json` | Config file path |
| `-out` | (per config file) | HTML report output path |
| `-filter` | (none) | Heading path substring filter |
| `-jobs` | `1` | Number of spec files to run in parallel |
| `-dry-run` | `false` | Parse and validate only |

## Configuration

specdown uses a data-only JSON configuration file.
The canonical config must not depend on any specific language runtime.

```json
{
  "title": "Project Spec",
  "include": ["specs/**/*.spec.md"],
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

| Field | Description |
|-------|-------------|
| `title` | Report title displayed as `<h1>`. Defaults to `"Specification"` |
| `include` | Glob patterns for spec files |
| `adapters` | List of adapters that handle executable blocks and fixtures |
| `reporters` | Output generators. `html` and `json` builtins provided |
| `models` | Alloy model verification. Can be omitted if not used |

### Missing include pattern

A config file without `include` must be rejected.

```verify:shell
mkdir -p .tmp-test
echo '{}' > .tmp-test/bad-config.json
! specdown run -config .tmp-test/bad-config.json 2>/dev/null
```

### Duplicate adapter name

Two adapters with the same name must be rejected.

```verify:shell
mkdir -p .tmp-test
cat <<'CFG' > .tmp-test/dup-adapter.json
{
  "include": ["*.spec.md"],
  "adapters": [
    {"name": "a", "command": ["true"], "blocks": ["run:x"]},
    {"name": "a", "command": ["true"], "blocks": ["run:y"]}
  ]
}
CFG
! specdown run -config .tmp-test/dup-adapter.json 2>/dev/null
```

## Document Structure

### Heading hierarchy

Heading hierarchy (`#`, `##`, `###`, ...) is converted into a test suite hierarchy.
Prose paragraphs are preserved in the HTML report but are not execution targets.

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
cat <<'CFG' > .tmp-test/timeout-cfg.json
{"include":["timeout.spec.md"],"adapters":[]}
CFG
specdown run -config .tmp-test/timeout-cfg.json -dry-run 2>&1
```

## Parsing

### Valid spec document

A well-formed spec document must parse without errors.

```run:shell
mkdir -p .tmp-test
cat <<'SPEC' > .tmp-test/valid.spec.md
# Valid Spec

Some prose.

## Section

More prose.
SPEC
cat <<'CFG' > .tmp-test/valid-cfg.json
{"include":["valid.spec.md"],"adapters":[]}
CFG
specdown run -config .tmp-test/valid-cfg.json -dry-run 2>&1
```

### Unclosed code block

A spec with an unclosed fenced code block must be rejected at parse time.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n```run:shell\necho hello\n' > .tmp-test/unclosed.spec.md
cat <<'CFG' > .tmp-test/unclosed-cfg.json
{"include":["unclosed.spec.md"],"adapters":[{"name":"s","command":["true"],"blocks":["run:shell"]}]}
CFG
! specdown run -config .tmp-test/unclosed-cfg.json 2>/dev/null
```

### Fixture without table

A fixture directive without parameters and not followed by a table must be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n<!-- fixture:x -->\n\nJust prose.\n' > .tmp-test/fnt.spec.md
cat <<'CFG' > .tmp-test/fnt-cfg.json
{"include":["fnt.spec.md"],"adapters":[{"name":"s","command":["true"],"blocks":["run:shell"],"fixtures":["x"]}]}
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
cat <<'CFG' > .tmp-test/fixture-call-cfg.json
{"include":["fixture-call.spec.md"],"adapters":[{"name":"s","command":["true"],"blocks":[],"fixtures":["check"]}]}
CFG
specdown run -config .tmp-test/fixture-call-cfg.json -dry-run 2>&1
```

### Hook directive without code block

A setup or teardown directive not followed by a code block must be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n<!-- setup:each -->\n\nJust prose.\n' > .tmp-test/hook-bad.spec.md
cat <<'CFG' > .tmp-test/hook-bad-cfg.json
{"include":["hook-bad.spec.md"],"adapters":[]}
CFG
! specdown run -config .tmp-test/hook-bad-cfg.json 2>/dev/null
```

### Hook directive with code block

A setup or teardown directive followed by an executable code block must parse successfully.

```run:shell
mkdir -p .tmp-test
printf '# Hook Test\n\n## Group\n\n<!-- setup:each -->\n```run:shell\necho init\n```\n\n### Scenario A\n\nSome prose.\n' > .tmp-test/hook-good.spec.md
cat <<'CFG' > .tmp-test/hook-good-cfg.json
{"include":["hook-good.spec.md"],"adapters":[{"name":"s","command":["true"],"blocks":["run:shell"]}]}
CFG
specdown run -config .tmp-test/hook-good-cfg.json -dry-run 2>&1
```

## Executable Blocks

Executable blocks are fenced code blocks whose info string starts with a recognized prefix.

| Prefix | Meaning |
|--------|---------|
| `run:<target>` | Side-effecting executable block |
| `verify:<target>` | Assertion block |
| `test:<name>` | Named high-level test DSL |

The `<target>` is defined by the adapter, not the core.

### Supported block kinds

The parser must recognize `run`, `verify`, and `test` as executable block kinds.

<!-- fixture:block-kind -->
| info | kind | target |
| --- | --- | --- |
| run:shell | run | shell |
| verify:api | verify | api |
| test:login | test | login |

### Variable capture

A block can capture its output into a variable with `-> $varName`.

<!-- fixture:block-kind -->
| info | kind | target |
| --- | --- | --- |
| run:shell -> $id | run | shell |

## Variables

Values captured from executable blocks are referenced in subsequent blocks
and tables using `${variableName}`.

### Scoping rules

- Variables from parent sections are readable in child sections
- Sibling sections at the same depth can share variables (in document order, only previously captured values)
- An unresolved variable is a compile-time error

### Variable escaping

To output a literal `${...}`, escape it with a backslash: `\${literal}`.

```run:shell -> $escapeTest
printf 'ok'
```

```verify:shell
test "${escapeTest}" = "ok"
```

### Unresolved variable error

Referencing a variable that was never captured must produce an error.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n```run:shell\necho \${missing}\n```\n' > .tmp-test/unresolved.spec.md
cat <<'CFG' > .tmp-test/unresolved-cfg.json
{"include":["unresolved.spec.md"],"adapters":[{"name":"s","command":["true"],"blocks":["run:shell"]}]}
CFG
specdown run -config .tmp-test/unresolved-cfg.json 2>&1 | grep -q "missing"
! specdown run -config .tmp-test/unresolved-cfg.json 2>/dev/null
```

## Fixture Tables

A Markdown table becomes executable when preceded by a fixture directive.

The directive is an HTML comment of the form `<!-- fixture:name -->`.
The first row is the header. Each subsequent row is an independent test case.
Fixture names are defined by the adapter, not the core.

### Cell escaping

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

### Fixture parameters

Fixtures can accept parameters via `(key=value)` syntax.
Parameters are passed to the adapter as `fixtureParams` in the `runCase` message.
Multiple parameters are comma-separated: `<!-- fixture:name(key1=val1, key2=val2) -->`.

### Parameterized fixture call

A fixture directive with parameters but no following table creates a single
assertion case. The adapter receives a `runCase` with `kind: "tableRow"`,
the fixture name, `fixtureParams` populated, and empty `columns`/`cells`.

This is useful for inline assertions that don't warrant a full table:

```markdown
<!-- fixture:check-user(field=plan, expected=STANDARD) -->
```

A fixture directive without parameters and without a table is a compile-time error.

## Setup / Teardown Hooks

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

## HTML Report

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
cat <<'CFG' > .tmp-test/report-test.json
{"title":"Report Test","include":["report-test.spec.md"],"adapters":[]}
CFG
specdown run -config .tmp-test/report-test.json -out report.html 2>&1 || true
```

The report file must exist and contain standard HTML structure.

```verify:shell
test -f .tmp-test/report.html
```

```verify:shell
grep -q '<html' .tmp-test/report.html
```

```verify:shell
grep -q 'Report Test' .tmp-test/report.html
```

### Artifacts

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
cat <<'CFG' > .tmp-test/diag.json
{"include":["diag.spec.md"],"adapters":[{"name":"d","command":["sh","./diag-adapter.sh"],"blocks":[],"fixtures":["diag"]}],"reporters":[{"builtin":"html","outFile":"diag-report.html"}]}
CFG
specdown run -config .tmp-test/diag.json 2>&1 || true
```

The JSON report must contain the expected and actual fields.

```verify:shell
grep -q '"expected"' .tmp-test/report.json
```

```verify:shell
grep -q '"actual"' .tmp-test/report.json
```

The adapter label must appear in the report.

```verify:shell
grep -q '"diag row"' .tmp-test/report.json
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
