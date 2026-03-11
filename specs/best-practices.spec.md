---
type: guide
---

# Best Practices

This guide collects patterns that work well in practice and mistakes
to avoid. Read the [explains::Spec Syntax](syntax.spec.md) and
[explains::Alloy Models](alloy.spec.md) chapters first.

## Document Structure

A spec file tells a story: what the system does, why, and how we know it works.

Lead with prose and design rationale.
Introduce Alloy models where structural properties matter.
Follow with executable blocks and check tables that confirm implementation.

### Keep Documents Focused

One spec file should cover one feature or one bounded concern.
Split by feature boundary, not by test type.
Do not put "all Alloy models" in one file and "all executable blocks" in another.
The value of specdown is that model and implementation verification live together.

## Alloy and Implementation: Complementary Roles

| Aspect | Alloy model | Executable block / check table |
|--------|-------------|----------------------------------|
| Abstraction | Design level | Implementation level |
| Coverage | Exhaustive within scope | Selected examples |
| Executes | Mathematical model | Actual code via adapter |
| Finds | Design flaws, missing constraints | Implementation bugs, integration issues |

Neither replaces the other. The power is in combining them in the same document.

## What Alloy Provides

Alloy's value comes from exhaustive exploration within a bounded scope.
The solver finds valid instances, counterexamples, and logical
relationships that are invisible to example-based testing alone.

### Verification

- **Valid instances** — `run` generates concrete instances satisfying the
  model's constraints, confirming the model is consistent (not vacuously
  true). A `run sanityCheck {}` should accompany every model.
- **Counterexample search** — `check` automatically finds assertion
  violations. Counterexamples are saved as artifacts for debugging.
- **Exhaustive coverage within scope** — executable blocks test selected
  examples; Alloy proves properties for every combination of atoms up to
  the given scope.

### Logical Relationship Discovery

- **Equivalence** — proving two models equivalent lets you reuse existing
  tests after a refactor (see [Equivalence Shield](#8-equivalence-shield)).
- **Implication** — proving a strong invariant implies weaker properties
  eliminates the need to test those weaker properties separately
  (see [Invariant Leverage](#4-invariant-leverage)).
- **Vacuous satisfaction detection** — if facts are contradictory, every
  assertion passes trivially. Alloy's instance finder exposes this trap.

### Document Quality

- **Design intent made precise** — an Alloy model forces prose claims
  into formal statements. Ambiguous prose becomes a concrete predicate.
- **Prose errors surfaced** — when a property claimed in prose fails in
  the model, the counterexample reveals the inconsistency immediately.
- **Missing constraints discovered** — when an executable block finds a
  bug, adding the missing constraint to the model lets Alloy search for
  further violations (see [Failure-Driven Modeling](#7-failure-driven-modeling)).

### Test Optimization

- **Exhaustive classification** — proving a case partition is complete
  and mutually exclusive means one representative per case suffices
  (see [Exhaustive Classification](#3-exhaustive-classification)).
- **Impossible transitions proved** — modeling state machines lets Alloy
  prove which transitions cannot occur, reducing the invalid-path tests
  needed (see [Transition Safety Net](#5-transition-safety-net)).
- **Composition coverage** — individual modules are tested with
  executable blocks; Alloy covers the combinatorial space of their
  interaction (see [Composition Safety](#6-composition-safety)).

## Patterns

### 1. Property and Implementation Side by Side

Place an `alloy:model` block with a `check` statement and a check table in the same section. Readers see both the design guarantee and the implementation confirmation together. When a `check` exists in the model block, the HTML report links the result automatically — no `alloy:ref` needed.

Here is a minimal example — an Alloy model proves that every item has an owner,
and a check table confirms the implementation enforces the same rule:

```alloy:model(ownership)
module ownership

sig User {}
sig Item { owner: one User }

assert everyItemHasOwner {
  all i: Item | one i.owner
}

check everyItemHasOwner for 5
```

```run:shell
# Verify the ownership model passes
mkdir -p .tmp-test
printf '%s\n' '# Ownership' '' '```alloy:model(ownership)' 'module ownership' '' 'sig User {}' 'sig Item { owner: one User }' '' 'assert everyItemHasOwner {' '  all i: Item | one i.owner' '}' '' 'check everyItemHasOwner for 5' '```' > .tmp-test/ownership.spec.md
printf '# T\n\n- [Own](ownership.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[],"models":{"builtin":"alloy"}}' > .tmp-test/ownership-cfg.json
specdown run -config .tmp-test/ownership-cfg.json 2>&1 | grep -q 'PASS'
```

### 2. Counterexample Harvesting

When Alloy finds a counterexample, fix the model, then add the counterexample as a check row to prevent regression. Document the counterexample in prose so future readers know why the row exists.

### 3. Exhaustive Classification

Use Alloy to prove the set of cases is complete and mutually exclusive,
then test one representative per case. Two assertions — one for
completeness, one for mutual exclusivity — together guarantee that
the check table covers every case with minimal rows.

Here a permission model partitions access into three levels.
Alloy proves every subject falls into exactly one level.

```alloy:model(classify)
module classify

abstract sig Level {}
one sig Admin, Member, Guest extends Level {}

sig Subject { level: one Level }

pred isAdmin[s: Subject] { s.level = Admin }
pred isMember[s: Subject] { s.level = Member }
pred isGuest[s: Subject] { s.level = Guest }

-- completeness: every subject is one of the three
assert complete {
  all s: Subject | isAdmin[s] or isMember[s] or isGuest[s]
}

-- mutual exclusivity: no overlap
assert exclusive {
  no s: Subject | (isAdmin[s] and isMember[s])
    or (isMember[s] and isGuest[s])
    or (isAdmin[s] and isGuest[s])
}

check complete for 5
check exclusive for 5
```

```run:shell
# Verify the classification model passes
mkdir -p .tmp-test
printf '%s\n' '# Classify' '' '```alloy:model(classify)' 'module classify' '' 'abstract sig Level {}' 'one sig Admin, Member, Guest extends Level {}' '' 'sig Subject { level: one Level }' '' 'pred isAdmin[s: Subject] { s.level = Admin }' 'pred isMember[s: Subject] { s.level = Member }' 'pred isGuest[s: Subject] { s.level = Guest }' '' 'assert complete {' '  all s: Subject | isAdmin[s] or isMember[s] or isGuest[s]' '}' '' 'assert exclusive {' '  no s: Subject | (isAdmin[s] and isMember[s])' '    or (isMember[s] and isGuest[s])' '    or (isAdmin[s] and isGuest[s])' '}' '' 'check complete for 5' 'check exclusive for 5' '```' > .tmp-test/classify.spec.md
printf '# T\n\n- [Classify](classify.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[],"models":{"builtin":"alloy"}}' > .tmp-test/classify-cfg.json
specdown run -config .tmp-test/classify-cfg.json 2>&1 | grep -q 'PASS'
```

With this proof, a check table needs only one representative per level
(Admin, Member, Guest) — Alloy guarantees there are no gaps.

### 4. Invariant Leverage

Prove that one strong invariant implies several weaker properties. Then only test the strong invariant in implementation checks and skip the rest. Document why the weaker tests are absent — the Alloy proofs serve as the justification.

### 5. Transition Safety Net

Model state transitions in Alloy to prove which transitions are
impossible. Executable blocks test the valid paths and a minimal set of
invalid ones. No need to test every impossible pair; Alloy covers the rest.

See the [State Machine Models](alloy.spec.md#state-machine-models)
section in the Alloy spec for a worked example.

### 6. Composition Safety

Test each module individually with executable blocks. Use Alloy to verify that their combination does not open unintended gaps. Testing every combination of states is impractical — Alloy covers the combinatorial space.

### 7. Failure-Driven Modeling

When an executable block discovers a bug, add the missing constraint to the Alloy model. Then let Alloy search for further violations. The cycle: implementation failure reveals a missing assumption, model is strengthened, Alloy finds more cases, new check rows are added.

### 8. Equivalence Shield

When refactoring, prove the old and new models are equivalent in Alloy. Then reuse existing executable blocks and check tables unchanged to confirm the implementation matches.

## Alloy Pitfalls

### Vacuous satisfaction

If the model's facts are contradictory, every assertion passes trivially. Always include `run sanityCheck {} for 5` to confirm the model has at least one instance. If it finds no instance, the model is inconsistent and all `check` results are meaningless.

### Missing `always` in temporal models

In temporal Alloy (models with `var` fields), `fact` and `assert` bodies are evaluated in the initial state only. To express a constraint that must hold in every state, wrap the body with `always`.

### Missing `var` when using prime

The prime operator (`e'`) refers to the value of `e` in the next state. Without `var`, the field is static and `e' = e` always holds, making the assertion vacuously true.

### Scope too small

`check ... for 3` searches only instances with up to 3 atoms per signature. Properties that only fail with 4+ interacting elements will not be caught. Use a scope of 5-7 as a practical default. For temporal models, also set an adequate step bound (e.g. `check ... for 5 but 8 steps`).

## Anti-Patterns

- **Model without implementation checks** — Alloy proves design properties but code may still be wrong. Always pair models with executable blocks or check tables.
- **Implementation checks without rationale** — future readers cannot tell which rows are essential. Add prose explaining why each case matters.
- **Alloy in a separate file** — defeats the purpose. Model and implementation checks should share the same section and prose context.
- **Over-modeling** — simple CRUD does not need Alloy. Use it when the state space is large enough that example-based testing cannot cover it.
- **Hardcoded paths in [explains::config](config.spec.md)** — use relative paths so the project works from any checkout location.
- **Monolithic [explains::adapter](adapter-protocol.spec.md)** — keep adapters focused on one execution environment. Split when complexity grows.

## Choosing the Right Tool

| Situation | Tool |
|-----------|------|
| Property must hold for all combinations | Alloy |
| API returns the right response | Executable block |
| Multiple input/output pairs to check | Check table |
| Refactoring safety | Alloy equivalence + existing checks |
| State reachability | Alloy |
| End-to-end workflow | Executable blocks in sequence |
