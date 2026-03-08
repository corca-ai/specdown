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

## Writing and Editing Specs

When you need to write or edit a spec file, first read `docs/guide-writing.md` for best practices and patterns. For syntax reference, find and read the spec syntax file (usually named `syntax.spec.md`).
