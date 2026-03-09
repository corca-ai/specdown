# Best Practices

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

## Patterns

### 1. Property and Implementation Side by Side

Place an `alloy:model` block with a `check` statement and a check table in the same section. Readers see both the design guarantee and the implementation confirmation together. When a `check` exists in the model block, the HTML report links the result automatically — no `alloy:ref` needed.

### 2. Counterexample Harvesting

When Alloy finds a counterexample, fix the model, then add the counterexample as a check row to prevent regression. Document the counterexample in prose so future readers know why the row exists.

### 3. Exhaustive Classification

Use Alloy to prove the set of cases is complete and mutually exclusive, then test one representative per case. Two assertions — one for completeness, one for mutual exclusivity — together guarantee that the check table covers every case with minimal rows.

### 4. Invariant Leverage

Prove that one strong invariant implies several weaker properties. Then only test the strong invariant in implementation checks and skip the rest. Document why the weaker tests are absent — the Alloy proofs serve as the justification.

### 5. Transition Safety Net

Model state transitions in Alloy to prove which transitions are impossible. Executable blocks test the valid paths and a minimal set of invalid ones. No need to test every impossible pair; Alloy covers the rest.

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

## Choosing the Right Tool

| Situation | Tool |
|-----------|------|
| Property must hold for all combinations | Alloy |
| API returns the right response | Executable block |
| Multiple input/output pairs to check | Check table |
| Refactoring safety | Alloy equivalence + existing checks |
| State reachability | Alloy |
| End-to-end workflow | Executable blocks in sequence |
