# Specdown

`specdown` is a Markdown-first executable specification system.
A single Markdown document serves as both a readable specification and an executable test.

## Version

The CLI must report its version when invoked with `version`.

```run:shell -> $version
specdown version
```

```verify:shell
echo "${version}" | grep -qE '^[a-z0-9]'
```

## Dry Run

Dry-run mode parses and validates spec files without executing adapters.
This is useful for checking syntax before a full run.

```run:shell -> $dryOutput
specdown run -config selfspec.json -dry-run 2>&1
```

The dry-run output must contain the word "spec" (listing discovered specs).

```verify:shell
echo "${dryOutput}" | grep -q "spec"
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

A fixture directive not followed by a table must be rejected.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n<!-- fixture:x -->\n\nJust prose.\n' > .tmp-test/fnt.spec.md
cat <<'CFG' > .tmp-test/fnt-cfg.json
{"include":["fnt.spec.md"],"adapters":[{"name":"s","command":["true"],"blocks":["run:shell"],"fixtures":["x"]}]}
CFG
! specdown run -config .tmp-test/fnt-cfg.json 2>/dev/null
```

## Configuration

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

## Block Syntax

### Supported block kinds

The parser must recognize `run`, `verify`, and `test` as executable block kinds.

<!-- fixture:block-kind -->
| info | kind | target |
| --- | --- | --- |
| run:shell | run | shell |
| verify:api | verify | api |
| test:login | test | login |

### Variable capture syntax

A block with `-> $varName` must capture the named variable.

<!-- fixture:block-kind -->
| info | kind | target |
| --- | --- | --- |
| run:shell -> $id | run | shell |

## Table Specs

### Cell escaping

Table cells support escape sequences that are processed by the core.

<!-- fixture:cell-escape -->
| input | expected |
| --- | --- |
| hello | hello |
| line1\nline2 | line1\nline2 |
| a\\\|b | a\\\|b |

## HTML Report

After a spec run, the HTML report must be generated as a self-contained file.

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

## Formal Properties

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
