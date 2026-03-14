---
name: specdown
description: Write, run, and fix specdown executable specifications. Use when the user asks to create, edit, run, or fix specs.
allowed-tools: Bash, Read, Edit, Glob, Grep
---

# specdown

Write, run, and fix executable specifications.

## Project Context

- Config: !`d="$PWD"; while [ "$d" != "/" ]; do if [ -f "$d/specdown.json" ]; then echo "path: $d/specdown.json"; cat "$d/specdown.json"; break; fi; d="$(dirname "$d")"; done; if [ "$d" = "/" ]; then echo "no specdown.json found"; fi`
- Specs: !`specdown run -dry-run 2>&1 | head -50`

## Authoring Specs

Before writing, read the reference specs below — they are the authoritative source of truth for specdown's syntax and behavior. Start with **Overview** and **Spec Syntax** for the basics.

1. Read the project's `specdown.json` to understand the entry file, adapters, and directory layout.
2. Create the `.spec.md` file in the appropriate directory.
3. Add it to the entry/index file: `- [Title](path/to/new.spec.md)`.
4. Structure: H1 title, prose context explaining the feature, then executable blocks and check tables that verify the implementation.
5. Run `specdown run` to verify.

Do NOT modify the spec unless the spec itself is wrong.

### Choosing the right verification approach

Before reaching for `run:shell`, check whether the project already has adapter-defined checks (`specdown.json` → `adapters[].checks`). Use the highest-level tool that fits:

| Situation | Use | Why |
|-----------|-----|-----|
| Project has a check that matches (e.g. `check:user-exists`) | `> check:name` table or inline `check:name(params)` | Document shows inputs and expected results only — no plumbing |
| Same `jq` / extraction pattern repeated 3+ times | Extract into an adapter check, then use `> check:name` table | Keeps specs clean; moves implementation detail into the adapter |
| One-off shell verification or setup | `run:shell` block | Simple and direct when not repeated |

Prefer check tables over `run:shell` + `jq` for public-facing specs — they read as data, not as scripts.

## Running and Fixing Specs

1. Run specs with `specdown run`. If $ARGUMENTS is provided, pass it as `-filter "$ARGUMENTS"`.
2. If all specs pass, report the result and stop.
3. If specs fail, read the failing spec file to understand the intent.
4. Fix the implementation to make the spec pass. Do NOT modify the spec unless the spec itself is wrong.
5. Re-run `specdown run` to confirm the fix.

Useful filters: `-filter type:alloy` (Alloy only), `-filter type:code` (code blocks only), `-filter type:table` (check tables only), `-filter block:shell` (shell blocks only), `-filter check:<name>` (specific check).

## Reference Specs

Each document below is itself a specdown spec — readable prose interleaved with executable examples. Read the ones relevant to your task.

### Getting started

| Spec | What it covers |
|------|----------------|
| [Overview](${CLAUDE_SKILL_DIR}/overview.md) | What specdown is, installation, project setup (`specdown init`), and a first-spec walkthrough showing how prose, executable blocks, and check tables work together |
| [Best Practices](${CLAUDE_SKILL_DIR}/best-practices.md) | How to structure a spec document (lead with prose, then verify), Alloy + implementation patterns (Invariant Leverage, Exhaustive Classification, Counterexample Harvesting, etc.), common pitfalls (vacuous satisfaction, scope too small), and anti-patterns to avoid |

### Writing specs

| Spec | What it covers |
|------|----------------|
| [Spec Syntax](${CLAUDE_SKILL_DIR}/syntax.md) | All executable elements: `run:<target>` blocks, doctest style (`$ ` lines with expected output), variable capture (`-> $var`) and scoping, `!fail` expected failures, wildcard matching (`...`), check tables (`> check:name`), check parameters, inline assertions (`expect:`, `check:`), setup/teardown hooks, summary lines, and YAML frontmatter (`timeout`) |
| [Configuration](${CLAUDE_SKILL_DIR}/config.md) | `specdown.json` format: entry file, adapter registration (`blocks`/`checks`), reporter configuration (HTML, JSON), Alloy model runner, global setup/teardown, defaults, `ignorePrefixes`, and validation rules that reject invalid config before scanning |
| [Validation Rules](${CLAUDE_SKILL_DIR}/validation.md) | Parse-time errors specdown catches before any adapter runs: unclosed code blocks, check without table, hook without code block, table without columns/rows, block without target |

### Adapters and execution

| Spec | What it covers |
|------|----------------|
| [Adapter Protocol](${CLAUDE_SKILL_DIR}/adapter-protocol.md) | NDJSON stdin/stdout process protocol: `exec` requests (run code, capture output) and `assert` requests (check table rows), response format (`output`/`error`, `passed`/`failed`), structured failure reporting (`expected`/`actual`/`label`), built-in shell adapter behavior, and complete adapter examples in Python and Shell |
| [CLI](${CLAUDE_SKILL_DIR}/cli.md) | Commands (`run`, `trace`, `init`, `alloy dump`, `install skills`), flags (`-config`, `-filter`, `-quiet`, `-dry-run`), filter expressions for targeting specific test types |

### Formal modeling

| Spec | What it covers |
|------|----------------|
| [Alloy Models](${CLAUDE_SKILL_DIR}/alloy.md) | Embedding `alloy:model(name)` blocks, `check`/`run` statements, `alloy:ref` directives for cross-section references, scoped checks with `but` clauses, state machine modeling with temporal operators, counterexample artifact generation, and combination rules for multi-fragment models |

### Advanced features

| Spec | What it covers |
|------|----------------|
| [Traceability](${CLAUDE_SKILL_DIR}/traceability.md) | Document-level traceability graph: typed documents (frontmatter `type`), named edges (`[edge::Title](target.md)`), `trace` config with `types`/`edges`/`ignore`, cardinality constraints (UML notation), cycle detection, transitive closure, strict mode, and integration with `specdown run` |
| [HTML Report](${CLAUDE_SKILL_DIR}/report.md) | Multi-page HTML report structure: sidebar with status dots, section-level pass/fail borders, failure diagnostics with expected/actual diffs, collapsed summary blocks |
| [Internals](${CLAUDE_SKILL_DIR}/internals.md) | Architecture for adapter authors and contributors: core/adapter/reporter separation, parallel execution model, design pillars |
