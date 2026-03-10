# Alloy Report Explainer

## Goal

Add a native toggle in the HTML live report so each `alloy:model(...)`
block can show a deterministic natural-language gloss of the Alloy source.
The gloss may be mechanical, but it must help a reader understand the model
without reading Alloy syntax first.

When an Alloy solver failure includes a counterexample, the report should
also show a deterministic natural-language summary of that counterexample
under the owning block.

## Constraints

- Do not use an LLM at report generation time.
- Keep the original Alloy source visible and authoritative.
- Keep the HTML report readable without JavaScript.
- Prefer block-local explanations over whole-model synthesis.
- Reuse existing report structure and failure styling where possible.

## Scope

### In Scope

- A toggle attached to each rendered `alloy:model(...)` block
- Deterministic gloss generation from the current block source
- Deterministic counterexample gloss generation from
  `AlloyCheckResult.Message`
- A minimal visible rendering for `alloy:ref(...)` that points to the
  owning model block when such an owner exists
- Reporter tests for the new UI and explanation text

### Out of Scope

- Full Alloy parsing or semantic proof-quality translation
- Cross-fragment global explanation synthesis
- LLM-backed or network-backed explanation generation
- New CLI flags or config surface for v1
- Direct parsing of the counterexample artifact file in the reporter

## UX

Each `alloy:model(...)` block keeps the current code presentation.
Below the code, the report adds a native disclosure section that reuses
the report's existing disclosure idiom instead of introducing a separate
button component.

- Summary label: `Explain alloy:model(<model>)`
- Body:
  - `Model` items for structural declarations in the block
  - `Rules` items for `fact` and `assert` bodies in the block
  - `Checks` items for `check ... for ...` commands in the block
  - `Counterexample` items when the block owns a failed check

The disclosure uses `<details>` and `<summary>`, styled to read like a
small secondary report disclosure. This preserves no-JS readability and
keeps the original source as the default visible content.

If the block owns a failed check, the disclosure opens by default so the
counterexample gloss is not hidden on first read.

## Explanation Rules

The explainer is intentionally narrow and mechanical.

### Structural Rules

- `module x` -> `Module name is x.`
- `sig A {}` -> `A is a signature.`
- `one sig A extends B {}` -> `Exactly one A exists, and A extends B.`
- `abstract sig A {}` -> `A is abstract.`
- Field line `board: one Board` -> `Each instance has exactly one board in Board.`
- Field line `next: lone Node` -> `Each instance has at most one next in Node.`

Multiplicity wording:

- `one` -> `exactly one`
- `lone` -> `at most one`
- `some` -> `one or more`
- `set` or omitted -> `zero or more`

### Rule and Assertion Rules

For common quantified forms:

- `all x: T | P` -> `For every x in T, P.`
- `some x: T | P` -> `There exists an x in T such that P.`
- `one x: T | P` -> `Exactly one x in T satisfies P.`
- `no x: T | P` -> `No x in T satisfies P.`

For common predicate fragments:

- `one x.f` -> `x.f has exactly one value`
- `no x.f` -> `x.f has no value`
- `x in Y` -> `x is in Y`
- `x = y` -> `x equals y`
- `P and Q` -> `P, and Q`
- `P implies Q` -> `if P, then Q`

Unknown expressions fall back to a safe gloss:

- `Constraint: <original expression>`

### Check Rules

- `check name for 5` -> `Check name is explored with scope 5.`
- `check name for 3 but 6 Int` ->
  `Check name is explored with default scope 3, and Int is widened to 6 bits.`

Scope wording:

- `for 5` means Alloy explores examples up to the default size 5 for each
  top-level signature unless overridden.
- Larger scope means Alloy searches a wider finite space, not an infinite
  proof.
- `but` means Alloy starts from the default scope and then overrides a
  named signature or solver setting such as `Int` or `steps`.
- `but 6 Int` should be glossed as `Int is widened to 6 bits` because that
  is the meaning highlighted in `selfspecs/alloy.spec.md`.

If a matching executed result exists for the same model, assertion, and
scope:

- passed -> append `Result: passed.`
- failed -> append `Result: failed.`

If no exact executed result exists, the gloss stays descriptive and does
not claim pass/fail for that local `check` line. This covers cases where an
explicit `alloy:ref(...)` suppresses the implicit inline check result.

## Counterexample Rules

The reporter relies on `AlloyCheckResult.Message` and does not parse the
counterexample artifact file directly in v1.

For v1:

1. Only attempt counterexample gloss generation when the failure is a solver
   counterexample:
   - `CounterexamplePath` is set, or
   - `Message` starts with `counterexample for `
2. Parse simple relation summaries from the existing message body.
3. Render short bullet lines:
   - `Card$0.board = Board$0` -> `Card$0 belongs to Board$0.`
   - `Node$0.next = Node$1` -> `Node$0 points to Node$1 through next.`
4. If a relation line cannot be classified, render:
   - `Observed relation: Card$0.board = Board$0`
5. If the failure is not a solver counterexample, render no counterexample
   gloss and keep the raw failure message as the primary diagnostic.

The raw failure message remains visible outside the disclosure, as today.

## Ownership Rules

The report must assign each Alloy result to exactly one rendered
`alloy:model(...)` block.

### Local Check Ownership

If a block source contains `check name for ...`, that block owns the
matching `AlloyCheckResult` for the same model, assertion name, and scope.

Checks are rendered in source order from the current block, not in map
iteration order.

### Fallback Ownership

If a result exists for a model but no rendered block in the document
declares that exact `check` locally, assign the result to the first
fragment of that model in document order.

This handles explicit `alloy:ref(...)` checks whose directive is rendered
elsewhere and avoids duplicating the same result across multiple fragments.

### Non-Owned Blocks

Blocks that do not own a result still render structural and rule glosses,
but they do not render a counterexample gloss for another block's check.

## `alloy:ref(...)` Visibility

`alloy:ref(...)` should not stay invisible in the report body.

For v1, render a compact report block at the reference location:

- label: `alloy:ref(model#assertion, scope=...)`
- status: pass, fail, or not executed
- link: when an owning model block exists, link to that block anchor with
  text such as `See model explanation`

This keeps cross-section failures discoverable without duplicating the full
explanation body.

## Implementation Plan

### Reporter

- Extend `renderDocument` so it prepares a stable ordered view of Alloy
  results plus a deterministic result-to-block ownership map before
  rendering nodes.
- Update `renderAlloyModel` to:
  - keep current code rendering
  - collect block-local checks declared in the block source
  - attach results by the ownership rules above
  - render checks and failures in stable source order
  - render a disclosure containing the generated gloss
  - avoid Go map iteration for choosing which Alloy failure to show
- Update `renderAlloyRef` to render a compact visible reference note and,
  when possible, a link to the owning model block

### Explainer

- Add reporter-local explainer helpers in the HTML reporter package
  because the feature is presentation-oriented.
- Keep parsing regex-based and line-oriented.
- Prefer explicit helper functions for:
  - declarations
  - fields
  - quantifiers
  - check commands
  - scope and `but` phrasing
  - counterexample relation lines from `AlloyCheckResult.Message`
- Fail soft on unsupported syntax:
  - preserve original expression text
  - never suppress the raw Alloy source
  - never suppress the raw failure message

### Tests

- Add reporter tests that verify:
  - the disclosure is rendered for Alloy blocks
  - the disclosure label includes the model name
  - failed owned checks auto-open the disclosure
  - declarations, assertions, and checks receive gloss text
  - failed checks render counterexample gloss text
  - non-counterexample Alloy failures do not invent counterexample glosses
  - dry-run or unknown Alloy status renders as not executed, not passed
  - unknown expressions fall back safely instead of disappearing
  - multiple checks in one block render in source order
  - mixed pass/fail checks remain deterministic
  - explicit `alloy:ref(...)` results are attached to exactly one model block
  - explicit `alloy:ref(...)` renders a visible link or note in the report body
  - no-JS disclosure structure exists for Alloy gloss sections
  - explicit `alloy:ref(...)` override of an inline `check` does not cause a
    false pass/fail claim on the inline check gloss

## Risks

- Alloy syntax is broad, so the explainer must fail soft.
- Model fragments can share the same logical model name, so result ownership
  must be explicit and deterministic.
- Overexplaining noisy syntax may hurt readability more than help it.
  The disclosure should stay compact and list-shaped.
- Dry-run and infrastructure failures produce non-passing Alloy results
  without counterexamples, so the explainer must not overclaim.

## Success Criteria

- A reader can expand an `Explain alloy:model(...)` section under each Alloy block.
- The original Alloy code remains visible without interaction.
- Common sample models in `selfspecs/alloy.spec.md` and `specs/pocket-card.spec.md`
  produce deterministic glosses.
- Failing Alloy checks show a readable counterexample gloss.
- A given Alloy result appears under exactly one model block.
- Cross-section `alloy:ref(...)` failures remain visible in the report body.
- `go test ./...` passes.

## Commit Strategy

1. Commit the design doc after review adjustments.
2. Commit reporter implementation and tests together.
3. Commit any post-review tidy/refactor changes separately if they are
   behavior-preserving.
