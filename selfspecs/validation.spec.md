# Validation Rules

specdown validates spec documents at parse time and rejects malformed input
before any adapter is invoked. The following errors are caught during parsing.

## Unclosed code block

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

## Fixture without table

A fixture directive without parameters and not followed by a table must be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n> fixture:x\n\nJust prose.\n' > .tmp-test/fnt.spec.md
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
> fixture:check(field=plan, expected=STANDARD)

More prose.
SPEC
printf '# T\n\n- [FC](fixture-call.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/fixture-call-cfg.json
{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":[],"fixtures":["check"]}]}
CFG
specdown run -config .tmp-test/fixture-call-cfg.json -dry-run 2>&1
```

## Hook without code block

A setup or teardown directive not followed by a code block must be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n> setup:each\n\nJust prose.\n' > .tmp-test/hook-bad.spec.md
printf '# T\n\n- [Hook](hook-bad.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/hook-bad-cfg.json
{"entry":"index.spec.md","adapters":[]}
CFG
! specdown run -config .tmp-test/hook-bad-cfg.json 2>/dev/null
```

A hook followed by a non-executable code block (e.g. plain `json`) must also be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n> setup\n\n```json\n{"a":1}\n```\n' > .tmp-test/hook-nonexec.spec.md
printf '# T\n\n- [HNE](hook-nonexec.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[]}' > .tmp-test/hook-nonexec-cfg.json
! specdown run -config .tmp-test/hook-nonexec-cfg.json 2>/dev/null
```

## Table structure

A table header must define at least one column.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n> fixture:x\n\n|||\n|---|\n|a|\n' > .tmp-test/table-nocol.spec.md
printf '# T\n\n- [TC](table-nocol.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"fixtures":["x"]}]}' > .tmp-test/table-nocol-cfg.json
! specdown run -config .tmp-test/table-nocol-cfg.json 2>/dev/null
```

A table must define at least one data row.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n> fixture:x\n\n| a |\n|---|\n' > .tmp-test/table-norow.spec.md
printf '# T\n\n- [TR](table-norow.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"fixtures":["x"]}]}' > .tmp-test/table-norow-cfg.json
! specdown run -config .tmp-test/table-norow-cfg.json 2>/dev/null
```

## Block specification

A block info string with a known prefix but no target must be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n```run:\necho hello\n```\n' > .tmp-test/no-target.spec.md
printf '# T\n\n- [NT](no-target.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[]}' > .tmp-test/no-target-cfg.json
! specdown run -config .tmp-test/no-target-cfg.json 2>/dev/null
```

Duplicate capture names in a single block must be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n```run:shell -> $a, $a\necho hello\n```\n' > .tmp-test/dup-capture.spec.md
printf '# T\n\n- [DC](dup-capture.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":["run:shell"]}]}' > .tmp-test/dup-capture-cfg.json
! specdown run -config .tmp-test/dup-capture-cfg.json 2>/dev/null
```

Doctest blocks do not support variable capture.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n```doctest:shell -> $x\n$ echo hi\nhi\n```\n' > .tmp-test/doctest-capture.spec.md
printf '# T\n\n- [DTC](doctest-capture.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":["doctest:shell"]}]}' > .tmp-test/doctest-capture-cfg.json
! specdown run -config .tmp-test/doctest-capture-cfg.json 2>/dev/null
```

`!fail` blocks do not support variable capture.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n```run:shell !fail -> $x\nexit 1\n```\n' > .tmp-test/fail-capture.spec.md
printf '# T\n\n- [FC](fail-capture.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":["run:shell"]}]}' > .tmp-test/fail-capture-cfg.json
! specdown run -config .tmp-test/fail-capture-cfg.json 2>/dev/null
```

## Alloy validation

An `alloy:ref` directive referencing an unknown model must be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n> alloy:ref(nonexistent#check, scope=5)\n' > .tmp-test/bad-ref.spec.md
printf '# T\n\n- [BR](bad-ref.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[],"models":{"builtin":"alloy"}}' > .tmp-test/bad-ref-cfg.json
! specdown run -config .tmp-test/bad-ref-cfg.json 2>/dev/null
```

## Unresolved variable

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
