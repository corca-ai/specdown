# Writing Good Specs

This guide covers best practices for writing effective specdown documents.
For syntax reference, see [Writing Specs](guide-spec.md).
For Alloy language details, see [Alloy Language Reference](alloy-reference.md).


## Document Structure

A spec file tells a story: what the system does, why, and how we know it works.

### Recommended Section Flow

```markdown
# Feature Name

Brief description of the feature and its purpose.

## Rules and Constraints

Prose explaining the design rationale.

(Alloy model fragments establishing formal properties)

## Behavior

(Executable blocks and fixture tables verifying implementation)
```

Lead with prose and design rationale.
Introduce Alloy models where structural properties matter.
Follow with executable blocks and fixture tables that confirm implementation.

### Keep Documents Focused

One spec file should cover one feature or one bounded concern.
A 300-line spec covering board CRUD is fine.
A 2000-line spec covering an entire product is not.

Split by feature boundary, not by test type.
Do not put "all Alloy models" in one file and "all executable blocks" in another.
The value of specdown is that model and implementation verification live together.


## Alloy and Implementation Verification: Complementary Roles

Alloy models and implementation checks (executable blocks and fixture tables) serve different purposes.

| Aspect | Alloy model | Executable block / fixture table |
|--------|-------------|----------------------------------|
| Abstraction | Design level | Implementation level |
| Coverage | Exhaustive within scope | Selected examples |
| Executes | Mathematical model | Actual code via adapter |
| Finds | Design flaws, missing constraints | Implementation bugs, integration issues |

Neither replaces the other. The power is in combining them in the same document.


## Patterns

### 1. Property and Implementation Side by Side

Place Alloy properties and implementation checks in the same section so readers see both the design guarantee and the implementation confirmation together.

````markdown
## Card Ownership

A card always belongs to exactly one board.

```alloy:model(board)
assert cardBelongsToOneBoard {
  all c: Card | one c.board
}
check cardBelongsToOneBoard for 5
```

<!-- alloy:ref(board#cardBelongsToOneBoard, scope=5) -->

<!-- fixture:card-exists -->
| board        | card      | exists |
|--------------|-----------|--------|
| ${boardName} | ${cardId} | yes    |
````

The `alloy:model` block defines and checks the assertion.
The `alloy:ref` directive links the check result to this section in the HTML report, displaying it as a status badge. Both are needed: the model block runs the check, the ref directive surfaces the result in context.

The fixture table confirms the implementation enforces the same property.

### 2. Counterexample Harvesting

When Alloy finds a counterexample, fix the model, then add the counterexample as a fixture row to prevent regression.

````markdown
## Card Movement

```alloy:model(board)
assert noMoveBetweenBoards {
  all c: Card, col: Column |
    move[c, col] implies col.board = c.column.board
}
check noMoveBetweenBoards for 5
```

<!-- alloy:ref(board#noMoveBetweenBoards, scope=5) -->

Counterexample found: archived columns lose board membership.
Fixed the model; pinning the case as a regression test.

<!-- fixture:move-card -->
| card   | target       | result | note                      |
|--------|--------------|--------|---------------------------|
| card-1 | same-board   | ok     | normal move               |
| card-1 | other-board  | reject | cross-board blocked        |
| card-1 | archived-col | reject | from counterexample        |
````

### 3. Exhaustive Classification

Use Alloy to prove the set of cases is complete and mutually exclusive, then test one representative per case.

````markdown
## Access Decision

```alloy:model(access)
assert exactlyFourCases {
  all u: User, p: Path |
    ownerAccess[u,p] or adminAccess[u,p] or publicAccess[u,p] or denied[u,p]
}

assert mutuallyExclusive {
  all u: User, p: Path {
    ownerAccess[u,p] implies not (adminAccess[u,p] or publicAccess[u,p] or denied[u,p])
    adminAccess[u,p] implies not (ownerAccess[u,p] or publicAccess[u,p] or denied[u,p])
    publicAccess[u,p] implies not (ownerAccess[u,p] or adminAccess[u,p] or denied[u,p])
  }
}

check exactlyFourCases for 6
check mutuallyExclusive for 6
```

<!-- alloy:ref(access#exactlyFourCases, scope=6) -->
<!-- alloy:ref(access#mutuallyExclusive, scope=6) -->

Four cases, proven complete. One representative per case is sufficient.

<!-- fixture:access-decision(user=alice) -->
| path             | decision | case         |
|------------------|----------|--------------|
| /private/alice/a | allow    | ownerAccess  |
| /private/alice/a | allow    | adminAccess  |
| /public/readme   | allow    | publicAccess |
| /private/alice/a | deny     | denied       |
````

Note: the fixture directive uses a parameter `(user=alice)` to set shared context for all rows. Parameters are passed to the adapter as `fixtureParams`. See [Writing Specs](guide-spec.md) for the parameter syntax.

### 4. Invariant Leverage

Prove that one strong invariant implies several weaker properties.
Then only test the strong invariant in implementation checks and skip the rest.

````markdown
## Column Ordering

```alloy:model(board)
pred positionInvariant[b: Board] {
  let cols = b.columns |
    cols.position = { i: Int | i >= 0 and i < #cols }
}

assert inv_implies_no_gaps {
  all b: Board | positionInvariant[b] implies noGaps[b]
}
assert inv_implies_no_duplicates {
  all b: Board | positionInvariant[b] implies noDuplicatePos[b]
}

check inv_implies_no_gaps for 6
check inv_implies_no_duplicates for 6
```

Both properties follow from positionInvariant.
Implementation checks only need to verify the invariant itself.

```verify:board
GET /boards/${boardId}/columns
positions = [0, 1, 2]
```
````

Document why gap and duplicate tests are absent.
The Alloy proofs serve as the justification.

### 5. Transition Safety Net

Model state transitions in Alloy to prove which transitions are impossible.
Executable blocks test the valid paths and a minimal set of invalid ones.

````markdown
## Card Lifecycle

<!-- setup -->

```run:api -> $cardId
POST /cards {"title": "test", "status": "draft"}
```

```alloy:model(card)
abstract sig Status {}
one sig Draft, Active, Archived, Deleted extends Status {}

sig Card { var status: one Status }

pred validTransition[c: Card, from, to: Status] {
  c.status = from and c.status' = to
}

assert deletedIsTerminal {
  always all c: Card |
    c.status = Deleted implies c.status' = Deleted
}

run sanityCheck {} for 5 but 8 steps
check deletedIsTerminal for 5 but 8 steps
```

<!-- alloy:ref(card#deletedIsTerminal, scope=5) -->

<!-- fixture:card-transition -->
| from     | to       | result |
|----------|----------|--------|
| Draft    | Active   | ok     |
| Active   | Archived | ok     |
| Active   | Deleted  | ok     |
| Deleted  | Active   | reject |
````

The last row confirms the terminal property at the implementation level.
No need to test every impossible pair; Alloy covers the rest.

**Temporal Alloy notes:**

- `var` makes a field mutable across states. Without it, the field is static and `status' = status` always holds, making temporal assertions vacuously true.
- The prime operator (`'`) refers to the value in the next state (e.g., `c.status'`).
- `always` makes the assertion hold in every state, not just the initial one. Without it, `assert` and `fact` bodies are evaluated only in the initial state.
- `but 8 steps` bounds the trace length. Without a step bound, temporal models default to very short traces.
- The `run sanityCheck` verifies the model is satisfiable before checking assertions (see [Pitfalls](#vacuous-satisfaction) below).

### 6. Composition Safety

Test each module individually with executable blocks.
Use Alloy to verify that their combination does not open unintended gaps.

````markdown
## Permissions and Quotas

### Permissions

```run:api -> $token
POST /auth/token {"user": "alice", "role": "editor"}
```

```verify:api
GET /docs/1 -H "Authorization: ${token}"
status = 200
```

### Quotas

```verify:api
GET /quota/alice
used < limit
```

### Combined Safety

```alloy:model(composed)
assert compositionSafe {
  all u: User, d: Doc |
    canAccess[u, d] implies
      (hasPermission[u, d] and withinQuota[u])
}
check compositionSafe for 6
```

<!-- alloy:ref(composed#compositionSafe, scope=6) -->
````

Testing every combination of permission and quota states is impractical.
Alloy covers the combinatorial space; executable blocks confirm each module works.

When a single spec uses multiple adapters (e.g., `run:api` and `run:shell`), each adapter handles only the block types it declared in `specdown.json`. The core routes requests automatically.

### 7. Failure-Driven Modeling

When an executable block discovers a bug, add the missing constraint to the Alloy model.
Then let Alloy search for further violations.

````markdown
## Concurrent Editing

Implementation testing revealed that simultaneous writes can corrupt content.

```alloy:model(edit)
sig Doc {
  var editor: lone User,
  var content: one Content
}

fact exclusiveEdit {
  always all d: Doc | lone d.editor
}

assert noCorruption {
  always all d: Doc |
    some d.editor implies after d.content != Corrupted
}

run sanityCheck {} for 5 but 8 steps
check noCorruption for 5 but 8 steps
```
````

`some d.editor` is the idiomatic way to say "the set is non-empty" in Alloy. The `after` operator is equivalent to prime (`'`) but reads more clearly in assertions.

The cycle: implementation failure reveals a missing assumption, model is strengthened, Alloy finds more cases, new fixture rows are added.

### 8. Equivalence Shield

When refactoring, prove the old and new models are equivalent in Alloy.
Then reuse existing executable blocks and fixture tables unchanged to confirm the implementation matches.

````markdown
## Permission Model v2

```alloy:model(migration)
pred canAccessV1[u: User, d: Doc] {
  u.role in d.allowedRoles
}

pred canAccessV2[u: User, d: Doc] {
  some a: u.attributes | a in d.requiredAttributes
}

assert v1_equiv_v2 {
  all u: User, d: Doc |
    canAccessV1[u, d] iff canAccessV2[u, d]
}
check v1_equiv_v2 for 6
```

Equivalence proven. Existing fixture tables and executable blocks remain valid for the v2 implementation.
````


## Alloy Pitfalls

### Vacuous satisfaction

If the model's facts are contradictory, every assertion passes trivially.
Always `run` an empty predicate first to confirm the model has at least one instance.

````markdown
```alloy:model(board)
run sanityCheck {} for 5
```
````

If `sanityCheck` finds no instance, the model is inconsistent and all `check` results are meaningless.

### Missing `always` in temporal models

In temporal Alloy (models with `var` fields), `fact` and `assert` bodies are evaluated in the initial state only.
To express a constraint that must hold in every state, wrap the body with `always`.

```alloy
-- Wrong: only constrains the initial state
fact { all d: Doc | lone d.editor }

-- Correct: constrains every state
fact { always all d: Doc | lone d.editor }
```

### Missing `var` when using prime

The prime operator (`e'`) refers to the value of `e` in the next state.
It only makes sense on mutable variables declared with `var`.
Without `var`, the field is static and `e' = e` always holds, making the assertion vacuously true.

### Scope too small

`check ... for 3` searches only instances with up to 3 atoms per top-level signature.
Properties that only fail with 4+ interacting elements will not be caught.

Use a scope of 5–7 as a practical default. For temporal models, also set an adequate step bound:

```alloy
-- Too small: may miss bugs
check deletedIsTerminal for 3

-- Practical default
check deletedIsTerminal for 5 but 8 steps
```

If a check is slow at scope 7, consider whether the model can be decomposed rather than reducing the scope.


## Anti-Patterns

### Model without implementation checks

An Alloy model that proves properties but has no executable blocks or fixture tables verifying the implementation. The model may be correct while the code is wrong.

### Implementation checks without rationale

A long fixture table with many rows but no prose explaining why these cases matter or how they relate to design properties. Future readers cannot tell which rows are essential.

### Alloy in a separate file

Putting all Alloy models in `models.spec.md` and all executable blocks in `tests.spec.md` defeats the purpose. The model and the implementation check should share the same section and the same prose context. See [Pattern 1](#1-property-and-implementation-side-by-side).

### Over-modeling

Not everything needs an Alloy model. Simple CRUD with no structural invariants is better served by executable blocks and fixture tables alone. Use Alloy when the state space is large enough that example-based testing cannot cover it.


## Choosing the Right Tool

| Situation | Tool |
|-----------|------|
| "Does this property hold for all combinations?" | Alloy |
| "Does this API return the right response?" | Executable block (`run:` / `verify:`) |
| "Are these the only possible cases?" | Alloy |
| "Does the implementation match for these inputs?" | Fixture table |
| "Is this refactoring safe?" | Alloy equivalence proof + existing checks ([Pattern 8](#8-equivalence-shield)) |
| "Is this state reachable?" | Alloy |
| "Does this workflow work end to end?" | Executable blocks in sequence |
| "Does this assertion hold inline?" | `expect` block |
