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

## Intent Captions

If the first line of a `run:`, `verify:`, or `test:` block is a comment,
specdown extracts it as the block's **intent caption**.

In the HTML report, blocks with a caption are rendered collapsed:
only the caption text and pass/fail indicator are visible. A `>` marker
on the right side lets readers expand the block to see the full code.
Failed blocks auto-expand so failures are never hidden.

This makes specs readable for non-technical stakeholders without
removing any detail for developers.

The comment prefixes recognized are `# `, `// `, and `-- `.
Only the text after the prefix becomes the caption;
the prefix itself and leading/trailing whitespace are stripped.

Doctest blocks never get captions — they use a different rendering model
with command/output pairs.

Here is an example: the following block's first line is a comment,
so the report will render it collapsed with the caption
"Demonstrate intent caption" and a pass/fail indicator.

```verify:shell
# Demonstrate intent caption
test 1 -eq 1
```

A block without a leading comment renders normally (not collapsed):

```verify:shell
test 1 -eq 1
```

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

Variables captured in a parent section are available in child sections.

```run:shell -> $parentVar
printf 'from-parent'
```

#### Child section

```verify:shell
test "${parentVar}" = "from-parent"
```

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

### Wildcard Matching

A line containing exactly `...` in the expected output matches zero or more
lines in the actual output. This is useful when output contains timestamps,
PIDs, temporary paths, or other values that change between runs.

```doctest:shell
$ echo hello && date && echo goodbye
hello
...
goodbye
```

A wildcard at the end matches any trailing output.

```doctest:shell
$ printf 'header\ndetail1\ndetail2'
header
...
```

A wildcard at the start matches any leading output.

```doctest:shell
$ printf 'preamble\nresult'
...
result
```

A lone `...` matches any output.

```doctest:shell
$ date
...
```

Multiple wildcards can appear in a single expected block.

```doctest:shell
$ printf 'a\nb\nc\nd\ne'
a
...
c
...
e
```

When no `...` line is present, matching is still exact (backward compatible).

```doctest:shell !fail
$ echo hello
world
```

## Expected Failures

Any executable block can be marked with `!fail` to indicate that failure
is the expected outcome. When the adapter reports failure as expected,
the case is rendered identically to a regular failure — red styling,
failure stats, and red dot markers in the ToC all apply.
The only difference is the exit code: a spec run exits 0
when expected failures are the only failures present.
If the adapter unexpectedly succeeds, the case is a real failure.

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

Parameters let one fixture definition handle many scenarios.
Instead of registering separate fixtures for each endpoint or mode,
register a single generic fixture and pass the differences as parameters:

```markdown
> fixture:check(endpoint=/api/users, mode=object)
| field | expected |
| name  | alice    |

> fixture:check(endpoint=/api/orders, mode=array, count=2)
| index | status  |
| 0     | SUCCESS |
| 1     | PENDING |
```

The adapter reads `fixtureParams.endpoint` and `fixtureParams.mode`
to decide how to fetch and validate, eliminating per-endpoint fixture code.

### Parameterized fixture call

A fixture directive with parameters but no following table creates a single
assertion case. The adapter receives a `runCase` with `kind: "tableRow"`,
the fixture name, `fixtureParams` populated, and empty `columns`/`cells`.

This is useful for inline assertions that don't warrant a full table:

```markdown
> fixture:check-user(field=plan, expected=STANDARD)
```

A fixture directive without parameters and without a table is a compile-time error.

## Inline Elements

Prose text can contain inline executable elements embedded in backtick code spans.
These are evaluated during the spec run and rendered with pass/fail status in the
HTML report.

### Prose variable rendering

Variables captured by earlier blocks can appear in prose text as `${name}`.
In the HTML report, resolved variables are displayed with their actual values
highlighted in green. Unresolved variables remain as literal `${name}` text.

```markdown
The greeting is ${greeting} and it was captured successfully.
```

### Inline expect

A backtick code span of the form `` `expect: EXPR == VALUE` `` creates an inline
equality assertion. Both sides support `${variable}` substitution. It counts
as a test case and appears green (pass) or red (fail) in the HTML report.

```markdown
The count is `expect: ${count} == 3` items.
```

For example, one plus one is `expect: 2 == 2`.

When the actual value does not match the expected value, the inline assertion
fails and the report shows both the actual value and the expected value.

Adding `!fail` at the end marks the assertion as an expected failure.
The inline value renders identically to a regular failure — red background,
red dot marker, and failure stats all apply. The only difference is
that expected failures do not cause a non-zero exit code.

This deliberately wrong assertion is an expected failure:
`expect: hello == goodbye !fail`.

### Inline fixture call

A backtick code span of the form `` `fixture:name(key=value)` `` creates an inline
fixture assertion. It reuses the adapter protocol with `kind: "tableRow"`,
the fixture name, and `fixtureParams` populated with empty `columns`/`cells`.

```markdown
The file `fixture:file-check(path=/tmp/data.txt, exists=yes)` was created.
```

When the adapter returns an `actual` value in its passed response, the inline
fixture displays the actual value as the main content with the fixture name
shown as a small ruby annotation above it.

For example, a + b is `fixture:echo-value(value=3)`.

Multiple inline elements can appear in the same paragraph.

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
