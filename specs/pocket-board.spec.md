# Pocket Board

`Pocket Board` is the toy project used to develop `specdown` incrementally.

Phase 0 started with a plain natural-language document.
Phase 1 keeps that prose intact and adds the first executable block.
The document still has no fixture tables, variables, or embedded models.

## Product Summary

`Pocket Board` is a tiny kanban board for personal work.
It has exactly three columns: `todo`, `doing`, and `done`.
The product is deliberately small so the specification can evolve without a large implementation burden.

## Why This Project Fits `specdown`

This project is small enough to understand in one sitting.
It still has enough structure to justify richer specification features in later phases.

Later phases can add:

- executable blocks for board commands
- fixture tables for transition rules
- embedded Alloy models for invariants
- richer HTML reporting with per-block and per-row status

## Core Concepts

A board contains cards.
Each card has an identifier, a title, and a current column.

Cards begin in `todo`.
Work in progress lives in `doing`.
Completed work lives in `done`.

## Behavioral Intent

The system should feel predictable.
Users should always be able to tell where a card is and what state it is in.

The board is expected to support a small set of valid transitions.
Those transitions are not formalized yet.
They will be turned into richer executable checks in later phases.

## Phase Status

Phase 0, Phase 1, Phase 2, and Phase 3 are complete in this repository.

The current implementation already does all of the following:

- finds this `.spec.md` file
- parses the document into headings, prose, and fenced code blocks
- dispatches `run:board` and `verify:board` through an external adapter command from `specdown.json`
- renders the document and block status into an HTML report
- shows failed cases in a summary section with links to the failing block
- returns a failing run result when one of the executable cases fails

## Next Target

This same document should be extended, not replaced.
Later phases can add fixtures, variables, and formal model fragments on top of the current foundation.

## First Executable Check

The first executable behavior is intentionally tiny.
The system should be able to create a board named `demo`.

```run:board
create-board "demo"
```

If this block executes successfully, `specdown` should emit a passing case result and show that result inline in the HTML report.

## Verify Created Board

Phase 3 adds the first assertion block.
After the previous command runs, the board named `demo` should exist.

```verify:board
board "demo" should exist
```

This block should pass and confirms that verification can read the state created by earlier `run:board` blocks.

## Missing Boards Fail Verification

Verification failures should be reported differently from command failures.
If a board does not exist, the assertion should fail with an expected-versus-actual style message.

```verify:board
board "archive" should exist
```

This block is intentionally failing.
`specdown run` should now exit non-zero, still write the HTML report, and show this failing verification in the summary with a link to the block.
