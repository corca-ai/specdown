# Guide: Iterative Spec Authoring with Alloy

Use `specdown alloy explore` to see what your Alloy models actually
say, then grow models and implementation tests together.

## Why

Writing a specification means writing down what you think the system
should do. Two problems arise:

1. What you wrote does not mean what you think it means.
2. Even when it does, it has consequences you did not expect.

Alloy's solver can show you both. A `run` command shows instances your
model allows. A `check` command finds counterexamples your model does
not prevent. But you have to **see** these results to learn from them.

`specdown alloy explore` surfaces this feedback. It runs only the Alloy
commands in your spec files and displays the actual instances — not
pass/fail, but the concrete worlds your model permits.

## The process

Model and implementation tests grow together. You do not finish the
model first and then write tests. Each discovery in the model becomes
an implementation test immediately.

```
model a fragment  -->  explore  -->  write an implementation test
       ^                                       |
       +---------------------------------------+
```

1. Write a small piece of model — a sig, a field, a fact.
2. Run `specdown alloy explore`.
3. Look at the instances. Ask: "is this what I intended?"
4. Write a check table row or executable block that tests the
   corresponding behavior in the real system.
5. Repeat from step 1.

This works the same way regardless of starting point:

- **New project**: start from `sig` and grow outward.
- **Existing system**: model what you think the system does. Explore
  to discover what your understanding actually permits. The model
  becomes a tool for understanding the existing system.
- **Existing specdown specs**: add Alloy models to existing specs.
  Explore to find assumptions that existing tests missed.

## Agent behavior

**This section is critical.** When assisting a user with this process,
the agent must follow these rules:

### Show, don't decide

The purpose of `explore` is to let the **user** see what the model
says and make judgment calls. The agent must not silently interpret
explore results and act on them.

After running `specdown alloy explore`:

1. **Show the full output** to the user.
2. **Ask** what is surprising or unintended. Do not assume.
3. **Wait** for the user to decide what to change before editing
   the model.

Wrong:

> I ran explore and found that the model allows X. I've added a
> constraint to prevent it and also fixed Y.

Right:

> Here are the instances explore found:
> ```
> (explore output)
> ```
> Instance 2 shows that a cancelled subscription can still have an
> active coupon. Is this intended, or should we add a constraint?

### Every model change must produce an implementation test

Do not change the model without also writing a corresponding check
table row or executable block. If the model gains a new fact, there
should be a new test verifying the real system enforces the same rule.
If a counterexample is found, there should be a test confirming the
implementation prevents it.

Wrong: making three model changes in a row, then moving on.

Right: one model change, one implementation test, then the next
model change.

If no adapter or check table exists yet for the relevant behavior,
say so and ask the user how to test it — do not skip the test.

### Keep the loop small

Each iteration should touch one thing:

- Add one sig/field/fact → explore → one test
- Add one assertion → explore → one test

Do not batch multiple model changes before exploring.

## Reading explore output

```
$ specdown alloy explore

spec: specs/access.spec.md

  model: access

    ✓ run sanityCheck {} for 5
      User$0
      Resource$0.owner = User$0
      Resource$1.owner = User$0

    ✗ check onlyOwnerReads for 5
      counterexample found:
      Resource$0.canRead = User$1
      Resource$0.owner = User$0
```

### Clearly wrong (binary — next action is obvious)

| Signal | Meaning | Action |
|--------|---------|--------|
| `run` finds no instances | Model is inconsistent — facts contradict each other | Relax a constraint |
| `check` finds a counterexample | Assertion does not hold | Strengthen the model or fix the assertion |
| No `run`/`check` commands in model | Cannot verify consistency | Add `pred sanityCheck {} run sanityCheck {} for 5` |

### Judgment needed (look at each instance)

When `run` finds instances, the model is consistent. Look at each
instance and ask whether it matches your intent. If an instance
should not be allowed, add a `fact` to exclude it, then explore again.

## Worked example

Suppose you are specifying a document access system.

**Step 1** — Write the basic structure:

````markdown
```alloy:model(access)
module access
sig User {}
sig Resource { owner: one User }
pred sanityCheck {}
run sanityCheck {} for 5
```
````

**Step 2** — Explore:

```
$ specdown alloy explore --filter access
```

You see instances where each Resource has exactly one owner. This
matches your intent — `one User` enforces it.

**Step 3** — Write a test: add a check table verifying that creating a
resource without an owner fails in the real system.

**Step 4** — Add a `canRead` field:

````markdown
```alloy:model(access)
sig Resource { owner: one User, canRead: set User }
```
````

**Step 5** — Explore again. You see an instance where `User$1` can read
`Resource$0` even though `User$0` owns it. Surprising — you expected
only owners to read.

**Step 6** — Add a constraint:

````markdown
```alloy:model(access)
fact { canRead in owner }
```
````

**Step 7** — Write a test: a check table row verifying that a non-owner
cannot read the resource in the real system.

**Step 8** — Add an assertion and explore:

````markdown
```alloy:model(access)
assert onlyOwnerReads { canRead in owner }
check onlyOwnerReads for 5
```
````

The check passes. Continue with the next aspect of the model.

At each step the model and the implementation tests grew together.
Each discovery became a concrete test.

## What explore shows

For each model, explore prints:

1. **Sigs** — the model's signature definitions with field types,
   multiplicities, and whether fields are mutable (`var`). Builtin
   sigs (univ, Int, String, none, seq/Int) are filtered out.

2. **Command results** — for each `run` and `check` command, the
   full instance JSON including `values`, `skolems`, and `state`
   (for temporal traces with multiple states).

Instance data is printed as raw JSON from the Alloy solver. Nothing
is filtered or reformatted — you see exactly what the solver produced.

When using `--repeat N`, each solution is labeled (`solution 1:`,
`solution 2:`, etc.). Multiple solutions show different valid
instances, giving a broader picture of what the model allows.

## CLI reference

### `specdown alloy explore [flags]`

Run Alloy models and display instance-level results. Only Alloy
commands execute — shell blocks, check tables, and adapters are
skipped.

| Flag | Description |
|------|-------------|
| `-filter <path>` | Only explore specs whose path contains the substring |
| `-model <name>` | Only explore the named model |
| `-repeat <N>` | Find N solutions per command (default: 1). More solutions show more of what the model allows |
| `-config <path>` | Path to specdown.json (default: `specdown.json`) |

### Relationship to `specdown run`

`explore` shares the same Alloy execution pipeline as `run` — model
synthesis, JAR invocation, receipt parsing. The differences:

| | `specdown run` | `specdown alloy explore` |
|---|---|---|
| **What executes** | Everything (Alloy, shell blocks, check tables) | Alloy commands only |
| **Output** | Pass/fail per case | Sigs + instances per command |
| **Purpose** | CI verification | Interactive model exploration |
