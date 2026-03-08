---
name: specdown
description: Run specdown specs, interpret failures, and fix them. Use when the user asks to run, check, or fix specs.
allowed-tools: Bash, Read, Edit, Glob, Grep
---

# specdown

Run executable specifications and fix failures.

## Project Context

- Config: !`cat specdown.json 2>/dev/null || echo "no specdown.json found"`
- Specs: !`specdown run -dry-run 2>&1 | head -50`

## Instructions

1. Run specs with `specdown run`. If $ARGUMENTS is provided, pass it as `-filter "$ARGUMENTS"`.
2. If all specs pass, report the result and stop.
3. If specs fail, read the failing spec file to understand the intent.
4. Fix the implementation to make the spec pass. Do NOT modify the spec unless the spec itself is wrong.
5. Re-run `specdown run` to confirm the fix.

## Spec Syntax Quick Reference

Spec files are `*.spec.md` Markdown documents. Prose is preserved as-is; only executable blocks, fixture tables, and Alloy model blocks are interpreted.

### Executable Blocks

Fenced code blocks with a recognized info string prefix:

```
run:<target>              — side-effecting block
verify:<target>           — assertion block
test:<name>               — named high-level test DSL
doctest:<target>          — inline command/output verification
```

Capture output into a variable: ` ```run:shell -> $varName `

Reference variables in subsequent blocks: `${varName}`

Mark expected failures with `!fail`: ` ```verify:shell !fail `

### Fixture Tables

A Markdown table preceded by `> fixture:name` directive. Each row is an independent test case.

```markdown
> fixture:user-exists(role=admin)

| name  | exists |
|-------|--------|
| alice | yes    |
| bob   | no     |
```

Parameters in parentheses are passed to the adapter as `fixtureParams`.

### Hooks

```markdown
> setup           — run before first case in section
> teardown        — run after last case in section
> setup:each      — run before each child section
> teardown:each   — run after each child section
```

A hook directive must be followed by an executable code block.

### Alloy Models

```markdown
```alloy:model(name)
sig Board { columns: set Column }
assert noOrphan { all c: Column | one c.~columns }
check noOrphan for 5
```
```

Fragments with the same model name are combined across sections. Use `> alloy:ref(name)` to display a check result from a different section.

## Writing Best Practices

### Document Structure

Lead with prose explaining design intent. Follow with Alloy models for structural properties, then executable blocks and fixture tables confirming implementation.

```markdown
# Feature Name

Brief description and rationale.

## Rules and Constraints

Prose + Alloy model fragments.

## Behavior

Executable blocks and fixture tables.
```

### Keep Specs Focused

One spec file per feature or bounded concern. Do not separate Alloy models and executable blocks into different files — the value is that they live together.

### Property and Implementation Side by Side

Place Alloy assertions and fixture tables in the same section so readers see both the design guarantee and the implementation confirmation together.

### Counterexample Harvesting

When Alloy finds a counterexample, fix the model, then add the counterexample as a fixture row to prevent regression.

### Choosing the Right Tool

| Situation | Tool |
|-----------|------|
| Property must hold for all combinations | Alloy |
| API returns the right response | Executable block |
| Multiple input/output pairs to check | Fixture table |
| Refactoring safety | Alloy equivalence + existing checks |
| End-to-end workflow | Executable blocks in sequence |

### Anti-Patterns to Avoid

- **Model without implementation checks** — Alloy proves design properties but code may still be wrong
- **Implementation checks without rationale** — readers cannot tell which rows are essential
- **Over-modeling** — simple CRUD does not need Alloy; use executable blocks and fixture tables
