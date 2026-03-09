# Alloy Models

Alloy fragments can be embedded directly in a spec document using
`alloy:model(name)` code blocks. This supports literate-style formal
verification — Alloy models are woven with natural language inside
the document.

## Embedding Rules

`alloy:model(name)` is a fragment belonging to the logical model `name`.
Fragments with the same model name are combined in document order
into a single logical model.

Only the first fragment may contain a `module` declaration.

When a model block contains a `check` statement for an assertion, the
reference is implicit — no separate directive is needed. The check result
is automatically displayed as a status badge in the HTML report.

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

A table row must not belong to more than one heading scope.

```alloy:model(docmodel)
assert rowBelongsToOneScope {
  all r: TableRow | one r.scope
}

check rowBelongsToOneScope for 5
```

## Scoped Checks with `but` Clauses

Alloy's `check` command supports type-specific scope overrides via
`but` clauses — for example, `check foo for 5 but 6 Int` sets the
default scope to 5 but widens the integer bitwidth to 6 bits.

This is essential when a model uses integer values outside the default
4-bit range (-8 to 7).

```alloy:model(scoring)
module scoring

sig Player {
  score: one Int
}

fact validScores {
  all p: Player | p.score >= 0 and p.score <= 21
}

assert scoreBound {
  all p: Player | p.score >= 0 implies p.score <= 21
}

check scoreBound for 3 but 6 Int
```

Without `but 6 Int`, Alloy's default 4-bit integers only cover -8 to 7,
which cannot represent values like 10 or 21. The wider bitwidth lets the
solver explore the full range declared in the fact.

```run:shell
mkdir -p .tmp-test
printf '# T\n\n- [But](but-scope.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[],"models":{"builtin":"alloy"},"reporters":[{"builtin":"json","outFile":"but-report.json"}]}' > .tmp-test/but-cfg.json
printf '%s\n' '# But Scope Test' '' '```alloy:model(scoring)' 'module scoring' '' 'sig Player {' '  score: one Int' '}' '' 'fact validScores {' '  all p: Player | p.score >= 0 and p.score <= 21' '}' '' 'assert scoreBound {' '  all p: Player | p.score >= 0 implies p.score <= 21' '}' '' 'check scoreBound for 3 but 6 Int' '```' > .tmp-test/but-scope.spec.md
specdown run -config .tmp-test/but-cfg.json 2>&1 || true
```

```verify:shell
grep -q '"status": "passed"' .tmp-test/but-report.json
```

## Combination Rules

Fragments with the same model name are merged into a single virtual `.als` file.
Source mapping comments are inserted into the generated model.

A `module` declaration in a subsequent fragment is a compile-time error.

```verify:shell
mkdir -p .tmp-test
printf '# Bad\n\n```alloy:model(dm)\nmodule dm\nsig A {}\n```\n\n```alloy:model(dm)\nmodule dm\nsig B {}\n```\n' > .tmp-test/dup-module.spec.md
printf '# T\n\n- [Dup](dup-module.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[],"models":{"builtin":"alloy"}}' > .tmp-test/dup-module-cfg.json
! specdown run -config .tmp-test/dup-module-cfg.json 2>/dev/null
```

## Model Reference

An explicit `alloy:ref` directive links a section to a check result
defined in a different section. This is useful for cross-section references.

```markdown
> alloy:ref(access#privateIsolation, scope=5)
```

The directive displays as a badge in the HTML report and links to
a counterexample artifact on failure.

A dry-run must recognize `alloy:ref` directives as alloy checks.

```run:shell -> $refOutput
mkdir -p .tmp-test
printf '# Ref Test\n\n## Model\n\n```alloy:model(rm)\nmodule rm\nsig A {}\nassert noOrphan { all a: A | a in A }\ncheck noOrphan for 3\n```\n\n## Reference\n\n> alloy:ref(rm#noOrphan, scope=5)\n' > .tmp-test/ref-test.spec.md
printf '# T\n\n- [Ref](ref-test.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[],"models":{"builtin":"alloy"}}' > .tmp-test/ref-test-cfg.json
specdown run -config .tmp-test/ref-test-cfg.json -dry-run 2>&1
```

```verify:shell
echo "${refOutput}" | grep -q "alloy:.*rm#noOrphan"
```

## Counterexample Artifacts

When an Alloy check fails, specdown writes a counterexample artifact to
`.artifacts/specdown/counterexamples/`. The `AlloyCheckResult` in the
JSON report includes a `counterexamplePath` field pointing to this file.

On failure, the `message` field includes a counterexample summary
extracted from the Alloy solver output.

```run:shell
mkdir -p .tmp-test
printf '%s\n' '# Counterexample Test' '' '```alloy:model(cx)' 'module cx' 'sig Node { next: lone Node }' 'assert allDisconnected { all n: Node | no n.next }' 'check allDisconnected for 3' '```' > .tmp-test/cx-test.spec.md
printf '# T\n\n- [CX](cx-test.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[],"models":{"builtin":"alloy"},"reporters":[{"builtin":"json","outFile":"cx-report.json"}]}' > .tmp-test/cx-cfg.json
specdown run -config .tmp-test/cx-cfg.json 2>&1 || true
```

The JSON report includes a `counterexamplePath` for the failing check.

```verify:shell
grep -q '"counterexamplePath"' .tmp-test/cx-report.json
```

The counterexample artifact file exists on disk.

```verify:shell
path=$(grep 'counterexamplePath' .tmp-test/cx-report.json | sed 's/.*"counterexamplePath"[^"]*"\([^"]*\)".*/\1/')
test -n "$path" && test -f "$path"
```
