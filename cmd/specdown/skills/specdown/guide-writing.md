# Best Practices

## Document Structure

A spec file tells a story: what the system does, why, and how we know it works.

Lead with prose and design rationale.
Introduce Alloy models where structural properties matter.
Follow with executable blocks and fixture tables that confirm implementation.

### Keep Documents Focused

One spec file should cover one feature or one bounded concern.
Split by feature boundary, not by test type.
Do not put "all Alloy models" in one file and "all executable blocks" in another.
The value of specdown is that model and implementation verification live together.

## Alloy and Implementation: Complementary Roles

| Aspect | Alloy model | Executable block / fixture table |
|--------|-------------|----------------------------------|
| Abstraction | Design level | Implementation level |
| Coverage | Exhaustive within scope | Selected examples |
| Executes | Mathematical model | Actual code via adapter |
| Finds | Design flaws, missing constraints | Implementation bugs, integration issues |

Neither replaces the other. The power is in combining them in the same document.

## Patterns

### 1. Property and Implementation Side by Side

Place an Alloy `check` and a fixture table in the same section. When a `check` exists in the model block, the HTML report links the result automatically — no `alloy:ref` needed.

````markdown
## Card Ownership

A card belongs to exactly one board.

```alloy:model(board)
assert cardOwnership { all c: Card | one c.board }
check cardOwnership for 5
```

> fixture:card-exists
| board | card | exists |
| --- | --- | --- |
| board-1 | card-1 | yes |
````

### 2. Counterexample Harvesting

When Alloy finds a counterexample, fix the model, then add the counterexample as a fixture row to prevent regression.

````markdown
Counterexample found: archived columns lose board membership.

> fixture:move-card
| card | target | result | note |
| --- | --- | --- | --- |
| card-1 | archived-col | reject | from counterexample |
````

### 3. Exhaustive Classification

Use Alloy to prove cases are complete and mutually exclusive, then test one representative per case.

````markdown
```alloy:model(access)
assert complete { all u: User, p: Path |
  owner[u,p] or admin[u,p] or public[u,p] or denied[u,p]
}
assert exclusive { all u: User, p: Path |
  owner[u,p] implies not (admin[u,p] or public[u,p] or denied[u,p])
  -- (same for each case)
}
check complete for 6
check exclusive for 6
```

Four cases, proven complete. One row per case is sufficient.

> fixture:access(user=alice)
| path | decision |
| --- | --- |
| /alice/private | allow |
| /public/readme | allow |
| /bob/private | deny |
````

### 4. Invariant Leverage

Prove one strong invariant implies several weaker properties. Only test the strong invariant; document why weaker tests are absent.

````markdown
```alloy:model(board)
assert inv_implies_no_gaps {
  all b: Board | positionInvariant[b] implies noGaps[b]
}
assert inv_implies_no_dupes {
  all b: Board | positionInvariant[b] implies noDuplicatePos[b]
}
check inv_implies_no_gaps for 6
check inv_implies_no_dupes for 6
```

Both follow from positionInvariant. Only verify the invariant:

```verify:board
GET /boards/${boardId}/columns
positions = [0, 1, 2]
```
````

### 5. Transition Safety Net

Model state transitions in Alloy to prove which are impossible. Test valid paths and a minimal set of invalid ones.

````markdown
```alloy:model(card)
abstract sig Status {}
one sig Draft, Active, Archived, Deleted extends Status {}
sig Card { var status: one Status }

assert deletedIsTerminal {
  always all c: Card | c.status = Deleted implies c.status' = Deleted
}
check deletedIsTerminal for 5 but 8 steps
```

> fixture:card-transition
| from | to | result |
| --- | --- | --- |
| Draft | Active | ok |
| Active | Archived | ok |
| Deleted | Active | reject |
````

### 6. Composition Safety

Test modules individually. Use Alloy to verify their combination has no unintended gaps. Testing every combination of states is impractical — Alloy covers the combinatorial space.

### 7. Failure-Driven Modeling

When an executable block discovers a bug, add the missing constraint to the Alloy model. Let Alloy search for further violations. The cycle: failure reveals a missing assumption, model is strengthened, Alloy finds more cases, new fixture rows are added.

### 8. Equivalence Shield

When refactoring, prove old and new models are equivalent. Reuse existing checks unchanged.

````markdown
```alloy:model(migration)
assert v1_equiv_v2 {
  all u: User, d: Doc | canAccessV1[u,d] iff canAccessV2[u,d]
}
check v1_equiv_v2 for 6
```
````

## Alloy Pitfalls

### Vacuous satisfaction

If the model's facts are contradictory, every assertion passes trivially. Always include `run sanityCheck {} for 5` to confirm the model has at least one instance. If it finds no instance, the model is inconsistent and all `check` results are meaningless.

### Missing `always` in temporal models

In temporal Alloy (models with `var` fields), `fact` and `assert` bodies are evaluated in the initial state only. To express a constraint that must hold in every state, wrap the body with `always`.

```alloy
-- Wrong: only constrains the initial state
fact { all d: Doc | lone d.editor }

-- Correct: constrains every state
fact { always all d: Doc | lone d.editor }
```

### Missing `var` when using prime

The prime operator (`e'`) refers to the value of `e` in the next state. Without `var`, the field is static and `e' = e` always holds, making the assertion vacuously true.

### Scope too small

`check ... for 3` searches only instances with up to 3 atoms per signature. Properties that only fail with 4+ interacting elements will not be caught. Use a scope of 5-7 as a practical default.

```alloy
-- Too small
check deletedIsTerminal for 3

-- Practical default
check deletedIsTerminal for 5 but 8 steps
```

## Anti-Patterns

- **Model without implementation checks** — Alloy proves design properties but code may still be wrong. Always pair models with executable blocks or fixture tables.
- **Implementation checks without rationale** — future readers cannot tell which rows are essential. Add prose explaining why each case matters.
- **Alloy in a separate file** — defeats the purpose. Model and implementation checks should share the same section and prose context.
- **Over-modeling** — simple CRUD does not need Alloy. Use it when the state space is large enough that example-based testing cannot cover it.

## Choosing the Right Tool

| Situation | Tool |
|-----------|------|
| Property must hold for all combinations | Alloy |
| API returns the right response | Executable block |
| Multiple input/output pairs to check | Fixture table |
| Refactoring safety | Alloy equivalence + existing checks |
| State reachability | Alloy |
| End-to-end workflow | Executable blocks in sequence |
