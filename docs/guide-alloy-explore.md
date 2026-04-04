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

## CLI reference

### `specdown alloy explore [flags]`

Run Alloy models and display instance-level results. Only Alloy
commands execute — shell blocks, check tables, and adapters are
skipped.

| Flag | Description |
|------|-------------|
| `-filter <path>` | Only explore specs whose path contains the substring |
| `-model <name>` | Only explore the named model |
| `-config <path>` | Path to specdown.json (default: `specdown.json`) |

### Relationship to `specdown run`

`explore` shares the same Alloy execution pipeline as `run` — model
synthesis, JAR invocation, receipt parsing. The differences:

| | `specdown run` | `specdown alloy explore` |
|---|---|---|
| **What executes** | Everything (Alloy, shell blocks, check tables) | Alloy commands only |
| **Output** | Pass/fail per case | Instances per command |
| **Purpose** | CI verification | Interactive model exploration |
