---
type: spec
---

# Spec Syntax

A spec file is plain Markdown. This document builds up the authoring surface
from simple to complex: headings, shell blocks, doctest blocks, variables,
check tables, hooks, and frontmatter.

## Headings and Prose

Heading hierarchy (`#`, `##`, `###`, ...) is converted into a test suite hierarchy.
Prose paragraphs are preserved in the HTML report but are not execution targets.

## Shell Blocks

`run:shell` fenced blocks are executable — specdown runs them and checks
for zero exit.

The parser must recognize `run` as the executable block kind.

| info | kind | target |
| --- | --- | --- |
| run:shell | run | shell |

Other prefixes (e.g. `verify:`, `test:`) are not recognized and produce
plain, non-executable code blocks. A spec containing only unrecognized
blocks has zero cases. Unrecognized prefixes emit a warning to stderr
so typos like `runn:shell` are caught early.

To suppress warnings for specific prefixes, add `ignorePrefixes` to `specdown.json`.
Plain info strings without a colon (e.g. `json`, `go`, `python`) never warn.

Any executable block can be marked with `!fail` to indicate that failure
is the expected outcome. When the adapter reports failure as expected,
the case is rendered identically to a regular failure — red styling,
failure stats, and red dot markers in the ToC all apply.
The only difference is the exit code: a spec run exits 0
when expected failures are the only failures present.
If the adapter unexpectedly succeeds, the case is a real failure.
`!fail` blocks do not support variable capture (`-> $name`).

```run:shell !fail
false
```

## Summary Lines

If the first line of a `run:` block is a comment,
specdown extracts it as the block's **summary line**.

Consecutive comment lines at the start of a block are joined with a
space into a single summary. The summary ends at the first non-comment
line or blank line.

In the HTML report, blocks with a summary are rendered collapsed:
only the summary text and pass/fail indicator are visible. A `>` marker
on the right side lets readers expand the block to see the full code.
Failed blocks auto-expand so failures are never hidden.

This makes specs readable for non-technical stakeholders without
removing any detail for developers.

The comment prefixes recognized are `# `, `// `, and `-- `.
Only the text after the prefix becomes the summary;
the prefix itself and leading/trailing whitespace are stripped.

Blocks with doctest content (`$ ` lines) never get summaries — they use
a different rendering model with command/output pairs.

Here is an example: the following block's first line is a comment,
so the report will render it collapsed with the summary
"Demonstrate summary line" and a pass/fail indicator.

```run:shell
# Demonstrate summary line
test 1 -eq 1
```

Multiple comment lines are joined into one summary:

```run:shell
# First part of the summary
# and the second part
test 1 -eq 1
```

A block without a leading comment renders normally (not collapsed):

```run:shell
test 1 -eq 1
```

## Doctest Blocks

A `run:<target>` block whose content starts with `$ ` lines is auto-detected
as doctest-style. Lines starting with `$ ` are commands; subsequent lines
until the next `$ ` or end of block are the expected output.

Commands are executed sequentially. On the first output mismatch, the block
fails with `expected` and `actual` values for diffing. Commands without
expected output lines are executed but only checked for exit status.

```run:shell
$ echo hello
hello
$ echo one two three
one two three
```

A doctest-style block with no expected output still verifies the command succeeds.

```run:shell
$ true
```

Multi-line expected output is matched exactly.

```run:shell
$ printf 'line1\nline2\nline3'
line1
line2
line3
```

Arithmetic and pipelines work as expected.

```run:shell
$ echo $((2 + 3))
5
$ seq 3 | tr '\n' '+' | sed 's/+$//'
1+2+3
```

Commands that produce no output show only the prompt line.

```run:shell
$ mkdir -p /tmp/specdown-test
$ touch /tmp/specdown-test/file.txt
$ test -f /tmp/specdown-test/file.txt
```

### Wildcard Matching

A line containing exactly `...` in the expected output matches zero or more
lines in the actual output. To match a literal `...` line, escape it as
`\...`. This is useful when output contains timestamps, PIDs, temporary
paths, or other values that change between runs.

A wildcard in the middle skips variable lines:

```run:shell
$ echo hello && date && echo goodbye
hello
...
goodbye
```

Multiple wildcards can appear in a single expected block:

```run:shell
$ printf 'a\nb\nc\nd\ne'
a
...
c
...
e
```

Escaped wildcard matches literal `...`:

```run:shell
$ echo '...'
\...
```

When no `...` line is present, matching is exact (backward compatible).

```run:shell !fail
$ echo hello
world
```

A doctest block with `!fail` intentionally shows the wrong expected output.
The `!fail` modifier makes the mismatch count as a pass.
On failure, passed steps render in green and the failing step renders
in red with the expected value below.

```run:shell !fail
$ echo hello
goodbye
```

When a case fails, remaining cases continue. Bindings from failed cases
are discarded.

## Variable Capture

A block can capture its output into one or more variables with `-> $varName`.
Multiple captures use comma-separated names: `-> $var1, $var2`.
Each output line is bound to the corresponding capture name in order.

| info | kind | target |
| --- | --- | --- |
| run:shell -> $id | run | shell |

Variables captured this way are referenced in subsequent blocks
and tables using `${variableName}`.

When the adapter returns structured (non-string) output, the value is
stored as-is and fields are accessible via dot-path syntax:
`${result.field}`. Nested access works to arbitrary depth:
`${result.outer.inner}`. Accessing a missing key or indexing into a
non-object value is a compile-time error.

### Scoping rules

- Variables from parent sections are readable in child sections
- Sibling sections at the same depth can share variables (in document order, only previously captured values)
- An unresolved variable is a compile-time error

The heading tree enforces a safety property: variables never leak
upward from child to parent sections.

```alloy:model(varscope)
module varscope

sig Section {
  parent: lone Section
}

sig Var {
  definedIn: one Section
}

-- heading hierarchy is a tree
fact tree {
  no s: Section | s in s.^parent
}

-- a variable is visible in its defining section and all descendants
pred visible[v: Var, s: Section] {
  v.definedIn in (s + s.^parent)
}

-- variables from child sections are never visible in ancestors
assert noUpwardLeak {
  all v: Var, s: Section |
    v.definedIn in s.^(~parent) implies not visible[v, s]
}

check noUpwardLeak for 6
```

Variables captured in a parent section are available in child sections.

```run:shell -> $parentVar
printf 'from-parent'
```

#### Child section

```run:shell
test "${parentVar}" = "from-parent"
```

When output has fewer lines than capture names, excess captures receive
empty string.

### Variable escaping

To output a literal `${...}`, escape it with a backslash: `\${literal}`.

```run:shell -> $escapeTest
printf 'ok'
```

```run:shell
test "${escapeTest}" = "ok"
```

## Check Tables

A Markdown table becomes executable when preceded by a check directive.

The directive is a blockquote of the form `> check:name`.
The first row is the header. Each subsequent row is an independent test case.
Check names are defined by the adapter, not the core.

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

| input | expected |
| --- | --- |
| hello | hello |
| line1\nline2 | line1\nline2 |
| a\\\|b | a\\\|b |

### Check parameters

Checks can accept parameters via `(key=value)` syntax.
Parameters are passed to the adapter as `checkParams` in the `assert` message.
Multiple parameters are comma-separated: `> check:name(key1=val1, key2=val2)`.

Parameters let one check definition handle many scenarios.
Instead of registering separate checks for each endpoint or mode,
register a single generic check and pass the differences as parameters:

```markdown
> check:api(endpoint=/api/users, mode=object)
| field | expected |
| name  | alice    |

> check:api(endpoint=/api/orders, mode=array, count=2)
| index | status  |
| 0     | SUCCESS |
| 1     | PENDING |
```

The adapter reads `checkParams.endpoint` and `checkParams.mode`
to decide how to fetch and validate, eliminating per-endpoint check code.

When a table column name matches a directive parameter name, the table
cell value takes precedence over the directive value. This allows
directive parameters to act as defaults that individual rows can
override.

### Parameterized check call

A check directive with parameters but no following table creates a single
assertion case. The adapter receives an `assert` message with
the check name, `checkParams` populated, and empty `columns`/`cells`.

This is useful for inline assertions that don't warrant a full table:

```markdown
> check:check-user(field=plan, expected=STANDARD)
```

A check directive without parameters and without a table is a compile-time error.

## Inline Elements

Prose text can contain inline executable elements embedded in backtick code spans.
These are evaluated during the spec run and rendered with pass/fail status in the
HTML report.

### Prose variable rendering

Variables captured by earlier blocks can appear in prose text as `${name}`.
In the HTML report, resolved variables are displayed with their actual values
highlighted in green. Referencing an undefined variable in prose is a
compile-time error, just like in executable blocks.

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

### Inline check call

A backtick code span of the form `` `check:name(key=value)` `` creates an inline
check assertion. It sends an `assert` message to the adapter with
the check name and `checkParams` populated, with empty `columns`/`cells`.

```markdown
The file `check:file-check(path=/tmp/data.txt, exists=yes)` was created.
```

When the adapter returns an `actual` value in its passed response, the inline
check displays the actual value as the main content with the check name
shown as a small ruby annotation above it.

For example, a + b is `check:jq(input=3, expr=., expected=3)`.

Multiple inline elements can appear in the same paragraph.

## Other Block Prefixes

The target in `run:<target>` names an [adapter](adapter-protocol.spec.md).
The built-in shell adapter works with no configuration; custom adapters
are registered in [`specdown.json`](config.spec.md).

| Prefix | Meaning |
|--------|---------|
| `run:<target>` | Executable block dispatched to an adapter |
| `alloy:model(<name>)` | Alloy model definition (see [Alloy](alloy.spec.md)) |
| `alloy:ref(<model>#<assertion>)` | Alloy model reference (see [Alloy](alloy.spec.md)) |

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

### Hook variable visibility

Hooks can read variables captured by blocks in parent sections.
When a hook executes, it receives all bindings visible at the
hook's heading path — the same scoping rules that apply to
regular blocks apply to hooks.

A setup or teardown directive followed by an executable code block must parse successfully.

```run:shell
# Verify spec with setup:each hook parses successfully
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
| `timeout` | Per-case execution time limit in milliseconds. Overrides `defaultTimeoutMsec` from config. `0` disables the time limit |
| `type` | Document type for [traceability](traceability.spec.md) (e.g. `spec`, `goal`, `feature`) |

If frontmatter is absent, the global `defaultTimeoutMsec` from `specdown.json` applies (default: 30 seconds).

A spec with a timeout must still pass when the adapter responds quickly.

```run:shell
# Verify spec with YAML frontmatter timeout parses successfully
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
