# Spec Syntax

This document builds up spec syntax from simple to complex:
headings, executable blocks, variables, fixture tables, hooks, and frontmatter.

## Headings and Prose

Heading hierarchy (`#`, `##`, `###`, ...) is converted into a test suite hierarchy.
Prose paragraphs are preserved in the HTML report but are not execution targets.

## Executable Blocks

Executable blocks are fenced code blocks whose info string starts with a recognized prefix.

| Prefix | Meaning |
|--------|---------|
| `run:<target>` | Side-effecting executable block |
| `verify:<target>` | Assertion block |
| `test:<name>` | Named high-level test DSL |
| `doctest:<target>` | Inline command/output verification block |

The `<target>` is defined by the adapter, not the core.

The parser must recognize `run`, `verify`, `test`, and `doctest` as executable block kinds.

> fixture:block-kind

| info | kind | target |
| --- | --- | --- |
| run:shell | run | shell |
| verify:api | verify | api |
| test:login | test | login |
| doctest:shell | doctest | shell |

## Variable Capture

A block can capture its output into a variable with `-> $varName`.

> fixture:block-kind
| info | kind | target |
| --- | --- | --- |
| run:shell -> $id | run | shell |

Variables captured this way are referenced in subsequent blocks
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

## Doctest Blocks

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

Arithmetic and pipelines work as expected.

```doctest:shell
$ echo $((2 + 3))
5
$ seq 3 | tr '\n' '+' | sed 's/+$//'
1+2+3
```

Commands that produce no output show only the prompt line.

```doctest:shell
$ mkdir -p /tmp/specdown-test
$ touch /tmp/specdown-test/file.txt
$ test -f /tmp/specdown-test/file.txt
```

## Expected Failures

Any executable block can be marked with `!fail` to indicate that failure
is the expected outcome. The spec passes when the adapter reports failure,
and fails if the adapter unexpectedly succeeds.

This is useful for documenting error cases, showing what invalid input
looks like, or including negative examples in a spec without breaking CI.

`!fail` blocks do not support variable capture (`-> $name`).

### Failing verify block

A command that exits non-zero is normally a failure. With `!fail`, it passes.

```verify:shell !fail
false
```

### Failing doctest with output mismatch

This doctest intentionally shows the wrong expected output.
The `!fail` modifier makes the mismatch count as a pass.

```doctest:shell !fail
$ echo hello
goodbye
```

### Failing run block

A run block that exits non-zero normally fails. With `!fail`, it passes.

```run:shell !fail
exit 1
```

### Multi-step doctest mismatch

When multiple commands are present, the block fails fast on the first
mismatch. Passed steps show actual output in green; the failed step
shows actual output in red with the expected value below it.

```doctest:shell !fail
$ echo first
first
$ echo second
WRONG
```

### Doctest with multi-line mismatch

Multi-line expected output is compared exactly. The entire actual
output is shown in red on mismatch.

```doctest:shell !fail
$ printf 'alpha\nbeta'
alpha
gamma
```

## Fixture Tables

A Markdown table becomes executable when preceded by a fixture directive.

The directive is a blockquote of the form `> fixture:name`.
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

> fixture:cell-escape
| input | expected |
| --- | --- |
| hello | hello |
| line1\nline2 | line1\nline2 |
| a\\\|b | a\\\|b |

### Fixture parameters

Fixtures can accept parameters via `(key=value)` syntax.
Parameters are passed to the adapter as `fixtureParams` in the `runCase` message.
Multiple parameters are comma-separated: `> fixture:name(key1=val1, key2=val2)`.

### Parameterized fixture call

A fixture directive with parameters but no following table creates a single
assertion case. The adapter receives a `runCase` with `kind: "tableRow"`,
the fixture name, `fixtureParams` populated, and empty `columns`/`cells`.

This is useful for inline assertions that don't warrant a full table:

```markdown
> fixture:check-user(field=plan, expected=STANDARD)
```

A fixture directive without parameters and without a table is a compile-time error.

## Setup and Teardown Hooks

Hooks run adapter commands at section boundaries.
A hook directive must be followed by an executable code block.

| Directive | Meaning |
|-----------|---------|
| `> setup` | Run once before the first case in the heading subtree |
| `> teardown` | Run once after the last case in the heading subtree |
| `> setup:each` | Run before the first case of each immediate child section |
| `> teardown:each` | Run after the last case of each immediate child section |

Hooks are not counted as test cases. Their results do not appear in the
case list, but a hook failure marks the document as failed.

A setup or teardown directive followed by an executable code block must parse successfully.

```run:shell
mkdir -p .tmp-test
printf '# Hook Test\n\n## Group\n\n> setup:each\n```run:shell\necho init\n```\n\n### Scenario A\n\nSome prose.\n' > .tmp-test/hook-good.spec.md
printf '# T\n\n- [Hook](hook-good.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/hook-good-cfg.json
{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":["run:shell"]}]}
CFG
specdown run -config .tmp-test/hook-good-cfg.json -dry-run 2>&1
```

## Frontmatter

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
