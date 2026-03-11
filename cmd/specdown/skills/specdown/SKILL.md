---
name: specdown
description: Run specdown specs, interpret failures, and fix them. Use when the user asks to run, check, or fix specs.
allowed-tools: Bash, Read, Edit, Glob, Grep
---

# specdown

Run executable specifications and fix failures.

## Project Context

- Config: !`cat specs/specdown.json 2>/dev/null || echo "no specs/specdown.json found"`
- Specs: !`specdown run -config specs/specdown.json -dry-run 2>&1 | head -50`

## Instructions

1. Run specs with `specdown run -config specs/specdown.json`. If $ARGUMENTS is provided, pass it as `-filter "$ARGUMENTS"`.
2. If all specs pass, report the result and stop.
3. If specs fail, read the failing spec file to understand the intent.
4. Fix the implementation to make the spec pass. Do NOT modify the spec unless the spec itself is wrong.
5. Re-run `specdown run -config specs/specdown.json` to confirm the fix.

## Using Alloy Models

Alloy is not optional decoration — it is a core part of specdown's value.
When writing or editing specs, actively look for opportunities to use Alloy:

- **Prove properties exhaustively** — executable blocks test selected examples; Alloy proves properties for all cases within scope. Use both.
- **Discover logical relationships** — Alloy can reveal equivalences (enabling test reuse after refactors) and implications (eliminating redundant tests).
- **Surface prose errors** — formalize prose claims as Alloy assertions. If the model finds a counterexample, the prose is wrong.
- **Optimize tests** — prove case partitions are complete, transitions are impossible, or compositions are safe, then test only what Alloy cannot cover.
- **Harvest counterexamples** — when Alloy finds a violation, add it as a check-table row to prevent regression.

Read `${CLAUDE_SKILL_DIR}/guide-writing.md` § "What Alloy Provides" for the full catalogue of benefits, and § "Patterns" for concrete recipes (Invariant Leverage, Exhaustive Classification, Failure-Driven Modeling, etc.).

Every model should include `run sanityCheck {} for 5` to guard against vacuous satisfaction.

### Fast Alloy Iteration

Use `-filter type:alloy` to run only Alloy checks without executing code blocks or check tables. This provides a fast feedback loop when writing or debugging models:

```
specdown run -config specs/specdown.json -filter type:alloy
```

Other useful filters: `type:code` (code blocks only), `type:table` (check tables only), `block:shell` (shell blocks only), `check:<name>` (specific check).

## Writing and Editing Specs

- Syntax: `${CLAUDE_SKILL_DIR}/syntax.md` — executable blocks, variables, check tables, hooks, frontmatter
- Configuration: `${CLAUDE_SKILL_DIR}/config.md` — specdown.json format, adapters, reporters, defaults
- Adapter protocol: `${CLAUDE_SKILL_DIR}/adapter-protocol.md` — NDJSON protocol, request/response format, examples
- Alloy models: `${CLAUDE_SKILL_DIR}/alloy.md` — embedding formal models, check statements, counterexamples
- Trace graph: `${CLAUDE_SKILL_DIR}/trace.md` — document traceability, typed edges, cardinality constraints
- Best practices: `${CLAUDE_SKILL_DIR}/guide-writing.md` — patterns, pitfalls, anti-patterns, **Alloy benefits and recipes**
