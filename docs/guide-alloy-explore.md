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

### Combining models to find edge cases

The most valuable discoveries often come not from modeling a single
rule, but from **combining** independently modeled rules. When two
aspects of the system (e.g., state transitions and payment retry)
are modeled separately, combine them into a single scenario and
explore. The solver may produce combinations that the spec never
explicitly addressed.

```
model A separately  -->  model B separately  -->  combine A + B  -->  explore
```

For example: a state transition model defines valid states, and a
retry model defines retry behavior. Neither mentions the other. But
when combined, the solver shows a user who is both "cancelled" and
"retrying a failed payment" — a scenario the spec never considered.
This is where explore earns its keep: surfacing implicit interactions
that humans miss because they think about rules one at a time.

## Agent behavior

**This section is critical.** When assisting a user with this process,
the agent must follow these rules strictly. The explore workflow is a
**collaborative conversation**, not a batch job. The user learns from
seeing what the model permits. The agent's role is to surface findings
and ask questions — never to make judgment calls on the user's behalf.

### Explain in plain language

The user may not know Alloy syntax or terminology. When showing
explore output or proposing model changes, always explain in terms
of the **business domain**, not Alloy mechanics.

- Do not say "sig", "pred", "fact", "relation", "atom", or "set"
  without immediately explaining what it means in the context of
  the system being modeled.
- Frame explore output as concrete scenarios: "This result shows
  a case where a user who cancelled their subscription transitions
  directly to the unregistering state" — not "Instance 2 shows
  `Cancelled.next = Unregistering`."
- When proposing a model change, describe the **business effect**
  first: "Right now the model doesn't show which states can
  transition to which. If we make the transitions explicit, explore
  will show concrete paths like 'subscribed → cancelled' or
  'cancelled → back to subscribed'."
- If the user asks a clarifying question about Alloy, answer it
  concisely with a domain example.

### Show, don't decide

The purpose of `explore` is to let the **user** see what the model
says and make judgment calls. The agent must not silently interpret
explore results and act on them.

After running `specdown alloy explore`:

1. **Show the full output** to the user.
2. **Point out** specific instances that look interesting, surprising,
   or potentially unintended — but frame these as questions, not
   conclusions.
3. **Wait** for the user to decide what to change before editing
   anything.

The agent must never:

- Decide what an instance means without asking the user.
- Add a constraint or assertion to "fix" something the user hasn't
  reviewed.
- Design a model fragment (new sigs, facts, assertions) independently
  and present it as a finished result.
- Run multiple explore → edit → explore cycles without user input
  between each cycle.

Wrong:

> I ran explore and found that the model allows X. I've added a
> constraint to prevent it and written three assertions to verify
> the boundary conditions.

Wrong (designing the model alone):

> I'll add a RetryState sig with elapsed days and two assertions
> to model the retry policy.

The agent designed the model, ran explore, confirmed it passed, and
presented the result. The user had no opportunity to see instances,
question assumptions, or make decisions. This defeats the purpose of
explore entirely.

Right:

> Here are the instances explore found:
> ```
> (explore output)
> ```
> Instance 2 shows that a cancelled subscription can still have an
> active coupon. Is this intended, or should we add a constraint?

Right (starting a new model area):

> The spec describes a 3-day retry policy for failed payments, but
> the Alloy model doesn't cover it yet. Want to start modeling that?
> If so, I'll add a minimal sig and we can explore what the solver
> generates before adding constraints.

After each explore, the agent shows output and asks. After the user
decides, the agent makes **one** change and explores again. This is
the rhythm of the workflow.

### Model, prose, and tests grow together

A spec document has three layers that must stay in sync:

1. **Prose** — natural language explaining the rule or behavior
2. **Alloy model** — formal constraints that the solver can verify
3. **Executable blocks** — check tables and shell blocks that
   test the real implementation

When the user decides to add or change a rule based on explore
findings, all three layers must be updated together:

- If a new fact is added to the model, the prose should explain
  the rule in natural language, and an executable block should
  verify the implementation enforces it.
- If a counterexample reveals a missing constraint, the prose
  should document why the constraint exists, and a test should
  confirm the implementation prevents the scenario.

Wrong: adding three Alloy assertions without touching prose or
executable blocks. The model grows but the document does not —
readers cannot understand what was discovered or why it matters.

Wrong: making three model changes in a row, then moving on.

Right: one model change, one prose update, one implementation test,
then the next iteration.

**Exception: when existing tests already cover the behavior.** If the
model formalizes a rule that is already tested by existing executable
blocks, a new test is not needed — the model adds a formal layer on
top of existing coverage. In this case, note which existing tests
cover the rule and move on. Only add new tests when the model reveals
behavior that is **not yet tested**.

If no adapter or check table exists yet for the relevant behavior,
say so and ask the user how to test it — do not skip the test.

### Keep the loop small

Each iteration should touch one thing:

- Add one sig/field/fact → explore → discuss → one prose + test update
- Add one assertion → explore → discuss → one prose + test update

Do not batch multiple model changes before exploring. Do not batch
multiple explorations before discussing with the user.

## What explore shows

For each model, explore prints:

1. **Sigs** — the model's signature definitions with field types,
   multiplicities, and whether fields are mutable (`var`). Builtin
   sigs (univ, Int, String, none, seq/Int) are filtered out. Shown
   as JSON from the receipt.

2. **Command results** — for each `run` and `check` command, the
   Alloy solver's text output showing all atoms and relation bindings.
   For temporal models, each trace state is shown separately. The
   text format uses Alloy's native notation:
   - `this/Sig={Atom$0, Atom$1}` — atoms in a signature
   - `this/Sig<:field={Atom$0->Atom$1}` — relation tuples
   - `skolem $var={Atom$0}` — skolemized variables

Instance data is the unmodified text output from the Alloy solver.
Nothing is filtered or reformatted.

When using `--repeat N`, each solution is labeled (`solution 1:`,
`solution 2:`, etc.). Multiple solutions show different valid
instances, giving a broader picture of what the model allows.

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

Suppose you are specifying a document access system. The agent and
user collaborate through a series of small explore-discuss-update
cycles. Each cycle touches model, prose, and tests together.

**Step 1** — Agent adds the basic structure and runs explore:

````markdown
```alloy:model(access)
module access
sig User {}
sig Resource { owner: one User }
pred sanityCheck {}
run sanityCheck {} for 5
```
````

```
$ specdown alloy explore --filter access
```

**Step 2** — Agent shows the output and asks:

> Explore found instances where each Resource has exactly one owner.
> This matches the `one User` constraint. Does this look right, or
> should a Resource be allowed to have no owner?

User confirms: "That's correct — every resource must have an owner."

**Step 3** — Agent updates all three layers:

- **Prose**: "Every resource must have exactly one owner."
- **Model**: (already expressed by `one User`)
- **Test**: a check table verifying that creating a resource without
  an owner fails in the real system.

**Step 4** — Agent adds a `canRead` field and explores:

````markdown
```alloy:model(access)
sig Resource { owner: one User, canRead: set User }
```
````

**Step 5** — Agent shows the output and asks:

> Instance 2 shows `Resource$0.canRead = User$1` while
> `Resource$0.owner = User$0`. This means a non-owner can read a
> resource. Is that intended, or should only owners be able to read?

User decides: "Only owners should read. Add a constraint."

**Step 6** — Agent updates all three layers:

- **Prose**: "Only the owner can read a resource."
- **Model**: `fact { canRead in owner }`
- **Test**: a check table row verifying that a non-owner cannot
  read the resource in the real system.

**Step 7** — Agent adds an assertion and explores:

````markdown
```alloy:model(access)
assert onlyOwnerReads { canRead in owner }
check onlyOwnerReads for 5
```
````

The check passes. Agent shows the result and asks if the user wants
to explore the next aspect.

At each step the user saw what the model permitted, made a judgment
call, and the agent updated prose, model, and tests together. No
step was skipped or batched.

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
