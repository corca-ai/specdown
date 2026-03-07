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

## Hook without code block

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
