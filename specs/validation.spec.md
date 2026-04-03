---
type: spec
workdir: .tmp-test
---

# Validation Rules

Malformed specs fail fast. specdown validates documents against the
[depends::spec syntax](syntax.spec.md) at parse time and rejects errors
before any adapter is invoked.

## Rules

| Rule | Rejection |
|------|-----------|
| Unclosed code block | Fenced block without closing `` ``` `` |
| Check without table | `> check:name` with no params and no table following |
| Hook without code block | `> setup` or `> teardown` not followed by a code block |
| Hook without executable block | Hook followed by a non-executable code block (e.g. ` ```json`) |
| Table needs columns | Header row must define at least one column |
| Table needs rows | At least one data row must follow the header |
| Table row/column mismatch | Data row has a different number of cells than the header |
| Block needs target | `run:` without a target name (e.g. `run:shell`) |
| Invalid capture syntax | Capture name doesn't match `$VarName` pattern |
| No duplicate captures | `-> $a, $a` on a single block |
| No capture on `!fail` | `!fail` and `-> $var` are mutually exclusive |
| Alloy ref exists | `alloy:ref` must reference a model defined in the same document |
| Alloy module redeclaration | A model fragment redeclares `module` after its first fragment |
| Invalid alloy ref syntax | Malformed `alloy:ref` directive |
| Variables resolved | Every `${name}` in blocks and prose must trace to a prior capture |

## Example: Validation Error

A representative validation error — a check directive without a following
table is rejected at parse time:

```run:shell
# Reject bare check directive with no table
printf '# Bad\n\n> check:x\n\nJust prose.\n' > fnt.spec.md
printf '# T\n\n- [Fnt](fnt.spec.md)\n' > index.spec.md
cat <<'CFG' > fnt-cfg.json
{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":["run:shell"],"checks":["x"]}]}
CFG
! specdown run -config fnt-cfg.json 2>/dev/null
```

Adding parameters makes the directive valid as a parameterized check call:

```run:shell
# Accept parameterized check call without table
cat <<'SPEC' > check-call.spec.md
# Check Call

Some prose.
> check:verify(field=plan, expected=STANDARD)

More prose.
SPEC
printf '# T\n\n- [FC](check-call.spec.md)\n' > index.spec.md
cat <<'CFG' > check-call-cfg.json
{"entry":"index.spec.md","adapters":[{"name":"s","command":["true"],"blocks":[],"checks":["verify"]}]}
CFG
specdown run -config check-call-cfg.json -dry-run 2>&1
```

All other validation rules are verified by unit tests.
