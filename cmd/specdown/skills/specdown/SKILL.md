---
name: specdown
description: Write, run, and fix specdown executable specifications. Use when the user asks to create, edit, run, or fix specs.
allowed-tools: Bash, Read, Edit, Glob, Grep
---

# specdown

Write, run, and fix executable specifications.

## Project Context

- Config: !`d="$PWD"; while [ "$d" != "/" ]; do if [ -f "$d/specdown.json" ]; then echo "path: $d/specdown.json"; cat "$d/specdown.json"; break; fi; d="$(dirname "$d")"; done; if [ "$d" = "/" ]; then echo "no specdown.json found"; fi`

## Workflows

Identify the user's scenario from the Project Context above, then **read the matching workflow guide before doing anything**.

| Scenario | How to detect | Guide |
|----------|---------------|-------|
| **New project** | No `specdown.json` found | [New Project](${CLAUDE_SKILL_DIR}/workflow-new-project.md) |
| **Adopting specdown** | `specdown.json` exists but few or no `.spec.md` files | [Adopt](${CLAUDE_SKILL_DIR}/workflow-adopt.md) |
| **Evolving specs** | Specs already exist; user wants to add, change, or strengthen them | [Evolve](${CLAUDE_SKILL_DIR}/workflow-evolve.md) |

## Running and Fixing Specs

1. Run specs with `specdown run`. If $ARGUMENTS is provided, pass it as `-filter "$ARGUMENTS"`. If the output is too long, add `-quiet`. Exit code 0 means all specs passed.
2. If all specs pass, report the result and stop.
3. If specs fail, read `report.json` in the configured output directory for structured diagnostics — this is better than piping output through `grep` or `tail`. Then read the failing spec file to understand the intent.
4. Use `-filter` to re-run only the relevant section while iterating on a fix.
5. Fix the implementation to make the spec pass. Do NOT modify the spec unless the spec itself is wrong.
6. Re-run specs to confirm the fix.

## Reference

**You must read the relevant reference docs before writing or modifying specs.** The descriptions below are for navigation — they do not contain enough detail to work from.

- [Overview](${CLAUDE_SKILL_DIR}/overview.md) — What specdown is, project setup with `specdown init`, and a first-spec walkthrough showing how prose, executable blocks, and check tables work together. Read this first if you haven't used specdown before.
- [Spec Syntax](${CLAUDE_SKILL_DIR}/syntax.md) — All executable elements: `run:<target>` blocks, doctest style (`$ ` lines with expected output), variable capture (`-> $var`) and scoping, `!fail` expected failures, wildcard matching (`...`), check tables (`> check:name`), inline assertions (`expect:`, `check:`), setup/teardown hooks, summary lines, and frontmatter fields (`timeout`, `type`, `workdir`). Read this before writing or editing any spec.
- [Best Practices](${CLAUDE_SKILL_DIR}/best-practices.md) — How to structure a spec document (lead with prose, then verify), choosing the right verification approach (doctest vs check table vs inline assertion vs shell block), Alloy modeling patterns, common pitfalls, and anti-patterns. Read this before writing or editing any spec.
- [Configuration](${CLAUDE_SKILL_DIR}/config.md) — `specdown.json` format: entry file, adapter registration (`blocks`/`checks`), reporter configuration, Alloy model runner, global setup/teardown, defaults, and `ignorePrefixes`. Read this when changing config or adding adapters.
- [Validation Rules](${CLAUDE_SKILL_DIR}/validation.md) — Parse-time errors specdown catches before any adapter runs: unclosed code blocks, check without table, hook without code block, table without columns/rows, block without target. Read this when debugging parse errors.
- [Adapter Protocol](${CLAUDE_SKILL_DIR}/adapter-protocol.md) — NDJSON stdin/stdout process protocol: `exec` requests (run code, capture output) and `assert` requests (check table rows), response format, structured failure reporting (`expected`/`actual`/`label`), and complete adapter examples in Python and Shell. Read this when building or debugging an adapter.
- [CLI](${CLAUDE_SKILL_DIR}/cli.md) — Commands (`run`, `trace`, `init`, `alloy dump`, `install skills`), flags (`-config`, `-filter`, `-quiet`, `-jobs`, `-max-failures`), and filter expressions (`type:`, `block:`, `check:` prefixes). Read this when you need to understand commands or flags.
- [Alloy Models](${CLAUDE_SKILL_DIR}/alloy.md) — Embedding `alloy:model(name)` blocks, `check`/`run` statements, `alloy:ref` cross-section references, scoped checks with `but` clauses, state machine modeling with temporal operators, and counterexample artifacts. Read this when working with formal models.
- [Traceability](${CLAUDE_SKILL_DIR}/traceability.md) — Document-level traceability graph: typed documents (frontmatter `type`), named edges (`[edge::Title](target.md)`), cardinality constraints, cycle detection, and strict mode. Read this when setting up document traceability.
- [HTML Report](${CLAUDE_SKILL_DIR}/report.md) — Multi-page HTML report structure, sidebar navigation, section-level pass/fail borders, failure diagnostics with expected/actual diffs, and `report.json` machine-readable output. Read this when customizing report output.
- [Internals](${CLAUDE_SKILL_DIR}/internals.md) — Architecture: core/adapter/reporter separation, parallel execution model, and design pillars. Read this when contributing to specdown itself.
