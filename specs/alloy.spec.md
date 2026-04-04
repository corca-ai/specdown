---
type: spec
---

# Alloy Models

[Alloy](https://alloytools.org/) is a lightweight formal modeling language.
Its solver exhaustively explores every combination of objects within a
bounded scope, finding counterexamples that example-based testing misses.

Specdown embeds Alloy models directly in spec files so that prose,
implementation checks, and formal proofs live side by side.
Executable blocks test selected examples; Alloy proves properties
for all cases within scope.

Alloy fragments are embedded using `alloy:model(name)`
[depends::code blocks](syntax.spec.md) and verified through the
[depends::Alloy runner](adapter-protocol.spec.md).
See [Best Practices](best-practices.spec.md) for patterns
on combining Alloy models with implementation checks.
For full Alloy language syntax and semantics, see `docs/alloy-reference.md`
in the repository root.

## Embedding Rules

`alloy:model(name)` is a fragment belonging to the logical model `name`.
Fragments with the same model name are combined in document order
into a single logical model. This lets a model grow incrementally
alongside the prose that motivates each piece, rather than appearing
as one monolithic block detached from context.

Only the first fragment may contain a `module` declaration — Alloy
requires exactly one `module` per compilation unit, and the first
fragment anchors it.

When a model block contains a `check` statement for an assertion, the
reference is implicit — no separate directive is needed. The check result
is automatically displayed as a status badge in the HTML report.

## Formal Properties

specdown's own document model provides a worked example.
Every executable unit belongs to exactly one heading scope,
and no two heading scopes can share an executable unit.

```alloy:model(docmodel)
module docmodel

sig Heading {}

sig Block {
  scope: one Heading
}

sig TableRow {
  scope: one Heading
}

pred sanityCheck {}
run sanityCheck {} for 5
```

The `one` multiplicity on `scope` already guarantees single ownership,
so the following assertions are provably true by construction.
In a real project you would model properties that are *not* obvious
from the signature alone — this example is deliberately simple to
introduce the syntax.

```alloy:model(docmodel)
assert blockBelongsToOneScope {
  all b: Block | one b.scope
}

check blockBelongsToOneScope for 5
```

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

pred sanityCheck {}
run sanityCheck {} for 3 but 6 Int
```

Without `but 6 Int`, Alloy's default 4-bit integers only cover -8 to 7,
which cannot represent values like 10 or 21. The wider bitwidth lets the
solver explore the full range declared in the fact.

## Combination Rules

Fragments with the same model name are merged into a single virtual `.als` file.
Source mapping comments are inserted into the generated model.

A `module` declaration in a subsequent fragment is a compile-time error.

## Model Reference

An explicit `alloy:ref` directive links a section to a check result
defined in a different section. This is useful for cross-section references.

```markdown
> alloy:ref(access#privateIsolation, scope=5)
```

The directive displays as a badge in the HTML report and links to
a counterexample artifact on failure.

## State Machine Models

Alloy is especially useful for modeling state machines where
exhaustive path coverage by example is impractical.

This example models a traffic light with valid transitions only.
Alloy proves that an invalid transition (Green → Red) cannot happen.

```alloy:model(traffic)
module traffic

abstract sig Color {}
one sig Red, Yellow, Green extends Color {}

sig Light {
  color: one Color
}

pred validTransition[from, to: Color] {
  from = Red implies to = Green
  from = Green implies to = Yellow
  from = Yellow implies to = Red
}

assert noGreenToRed {
  all from, to: Color |
    validTransition[from, to] implies not (from = Green and to = Red)
}

check noGreenToRed for 5

pred sanityCheck {}
run sanityCheck {} for 5
```

## Counterexample Artifacts

When an Alloy check fails, specdown writes a counterexample artifact to
`.artifacts/specdown/counterexamples/`. The alloy result detail in the
JSON report includes a `counterexamplePath` field (at `.alloy.counterexamplePath`
within a case result) pointing to this file.

On failure, the `message` field includes a counterexample summary
extracted from the Alloy solver output, and the artifact file is
written to disk.
