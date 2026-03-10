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

## Writing and Editing Specs

- Syntax: `${CLAUDE_SKILL_DIR}/syntax.md` — executable blocks, variables, check tables, hooks, frontmatter
- Configuration: `${CLAUDE_SKILL_DIR}/config.md` — specdown.json format, adapters, reporters, defaults
- Adapter protocol: `${CLAUDE_SKILL_DIR}/adapter-protocol.md` — NDJSON protocol, request/response format, examples
- Alloy models: `${CLAUDE_SKILL_DIR}/alloy.md` — embedding formal models, check statements, counterexamples
- Trace graph: `${CLAUDE_SKILL_DIR}/trace.md` — document traceability, typed edges, cardinality constraints
- Best practices: `${CLAUDE_SKILL_DIR}/guide-writing.md` — patterns, pitfalls, anti-patterns
