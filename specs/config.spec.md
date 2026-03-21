---
type: spec
---

# Configuration

Every project needs a `specdown.json`. It tells specdown where specs live,
which [depends::adapters](adapter-protocol.spec.md) to launch, and what
[reporters](report.spec.md) to generate.

Place `specdown.json` at the project root, next to `.git/`. All paths
inside the config are resolved relative to the config file's directory,
so a root-level config can reference specs in any subdirectory (e.g.
`"entry": "docs/specs/index.spec.md"`). This also makes the config easy
to find for both humans and tools — `specdown run` looks for
`specdown.json` in the current directory by default.

The config is data-only JSON — no scripting, no language runtime dependency.
For v1, a single file is sufficient.

```json
{
  "entry": "specs/index.spec.md",
  "adapters": [
    {
      "name": "myapp",
      "command": ["python3", "./tools/adapter.py"],
      "blocks": ["run:myapp"],
      "checks": ["user-exists"]
    }
  ]
}
```

The `models` and `reporters` fields can be included to override defaults.
See [Defaults](#defaults).

## Entry File

The `entry` field points to a Markdown file that serves as the starting point for recursive crawling.
If it has an H1 heading, that becomes the entry page title.
Markdown links to `.md` and `.spec.md` files are followed recursively to discover all pages.

```run:shell
# Generate report from entry file and verify title
mkdir -p .tmp-test
cat <<'SPEC' > .tmp-test/entry-test.spec.md
# My Feature

Some prose.
SPEC
printf '# My Project Title\n\n- [Feature](entry-test.spec.md)\n' > .tmp-test/entry-index.spec.md
cat <<'CFG' > .tmp-test/entry-test-cfg.json
{"entry":"entry-index.spec.md","adapters":[],"reporters":[{"builtin":"html","outFile":"entry-report"}]}
CFG
specdown run -config .tmp-test/entry-test-cfg.json 2>&1 || true
```

The H1 heading from the entry file appears as that page's title.

```run:shell
$ grep -o '<title>[^<]*</title>' .tmp-test/entry-report/entry-index.html
<title>My Project Title</title>
```

## Built-in Shell Adapter

The shell adapter is built into specdown. `run:shell` blocks
work without any adapter configuration.

```run:shell
# Run a spec using the built-in shell adapter with no adapter config
mkdir -p .tmp-test
printf '# T\n\n- [S](builtin-shell-test.spec.md)\n' > .tmp-test/builtin-shell-index.spec.md
BT=$(printf '\140\140\140')
printf '%s\n' '# Builtin Shell' '' "$BT"'run:shell' '$ echo works' 'works' "$BT" > .tmp-test/builtin-shell-test.spec.md
printf '{"entry":"builtin-shell-index.spec.md","adapters":[]}' > .tmp-test/builtin-shell-cfg.json
specdown run -config .tmp-test/builtin-shell-cfg.json 2>&1 || true
```

```run:shell
$ specdown run -config .tmp-test/builtin-shell-cfg.json 2>&1 | grep '^PASS' | sed 's/ in [0-9]*ms//'
PASS 2 spec(s), 1 case(s)
```

If a user adapter explicitly claims a shell block (e.g., `"blocks": ["run:shell"]`),
the user adapter takes precedence over the built-in.

## Built-in jq Check

The `check:jq` check is built into specdown. It evaluates
[jq](https://jqlang.github.io/jq/) expressions against JSON data and
compares the result with an expected value. No adapter configuration is
required.

Columns (or check parameters):

| Column | Description |
|--------|-------------|
| `input` | JSON string to evaluate against |
| `expr` | jq expression |
| `expected` | Expected result |

Use `input` as a check parameter when every row operates on the same JSON:

```run:shell -> $jq_data
echo '{"name":"Alice","age":30,"tags":["admin","user"]}'
```

> check:jq(input=${jq_data})
| expr | expected |
|------|----------|
| .name | Alice |
| .age | 30 |
| .tags | ["admin","user"] |

Or use `input` as a column when rows have different inputs:

```run:shell -> $jq_other
echo '{"city":"Seoul"}'
```

> check:jq
| input | expr | expected |
|-------|------|----------|
| ${jq_data} | .name | Alice |
| ${jq_other} | .city | Seoul |

Full jq expressions are supported — pipes, filters, and boolean conditions
all work:

> check:jq(input=${jq_data})
| expr | expected |
|------|----------|
| .tags \| length | 2 |
| .age > 18 | true |

Array and object comparisons are whitespace-insensitive and key-order-insensitive.

If a user adapter explicitly claims `check:jq`, the user adapter takes
precedence over the built-in.

## Config Fields

| Field | Description |
|-------|-------------|
| `entry` | Path to the entry Markdown file. Starting point for recursive link crawling |
| `adapters` | List of adapters that handle executable blocks and checks |
| `reporters` | Output generators. `html` and `json` builtins provided |
| `models` | Alloy model verification (default: `alloy`). Accepted for explicit configuration; omit to use the default |
| `ignorePrefixes` | List of code block prefixes to suppress unknown-prefix warnings for |
| `trace` | Traceability configuration. See [Traceability](traceability.spec.md) |
| `toc` | Sidebar table-of-contents grouping. See [TOC Grouping](#toc-grouping) below |
| `setup` | Shell command to run once before any specs execute |
| `teardown` | Shell command to run once after all specs finish (runs even on failure) |
| `defaultTimeoutMsec` | Default adapter request timeout in milliseconds (default: `30000`). `0` disables the time limit |

## Global Setup and Teardown

The `setup` and `teardown` fields run shell commands before and after the
entire spec run. This is useful for managing test infrastructure — starting
databases, launching containers, seeding data, or cleaning up afterwards.

```json
{
  "setup": "docker compose up -d && sleep 2",
  "teardown": "docker compose down",
  "entry": "specs/index.spec.md",
  "adapters": []
}
```

The setup command runs before any spec execution begins. If it fails,
specdown exits immediately without running specs.

```run:shell
# Verify setup runs before specs
mkdir -p .tmp-test/setup-test
printf '# T\n\n- [S](s.spec.md)\n' > .tmp-test/setup-test/index.spec.md
printf '# S\n\nProse.\n' > .tmp-test/setup-test/s.spec.md
cat <<'CFG' > .tmp-test/setup-test/specdown.json
{"entry": "index.spec.md", "setup": "echo SETUP-RAN > setup-marker.txt"}
CFG
specdown run -config .tmp-test/setup-test/specdown.json 2>&1 || true
```

```run:shell
$ cat .tmp-test/setup-test/setup-marker.txt
SETUP-RAN
```

The teardown command runs after specs complete, even if specs fail.

```run:shell
# Verify teardown runs after specs (even on failure)
mkdir -p .tmp-test/teardown-test
printf '# T\n\n- [S](s.spec.md)\n' > .tmp-test/teardown-test/index.spec.md
BT=$(printf '\140\140\140')
printf '%s\n' '# S' '' "$BT"'run:shell' '$ echo hello' 'wrong-output' "$BT" > .tmp-test/teardown-test/s.spec.md
cat <<'CFG' > .tmp-test/teardown-test/specdown.json
{"entry": "index.spec.md", "setup": "echo SETUP-OK > marker.txt", "teardown": "echo TEARDOWN-RAN >> marker.txt"}
CFG
specdown run -config .tmp-test/teardown-test/specdown.json 2>&1 || true
```

```run:shell
$ cat .tmp-test/teardown-test/marker.txt
SETUP-OK
TEARDOWN-RAN
```

A failing setup command prevents spec execution and exits with an error.

```run:shell
# Verify failing setup aborts the run
mkdir -p .tmp-test/setup-fail-test
printf '# T\n\n- [S](s.spec.md)\n' > .tmp-test/setup-fail-test/index.spec.md
printf '# S\n\nProse.\n' > .tmp-test/setup-fail-test/s.spec.md
cat <<'CFG' > .tmp-test/setup-fail-test/specdown.json
{"entry": "index.spec.md", "setup": "exit 1"}
CFG
! specdown run -config .tmp-test/setup-fail-test/specdown.json 2>/dev/null
```

### Adapter Fields

| Field | Description |
|-------|-------------|
| `name` | Unique identifier for the adapter |
| `command` | Array of strings — the executable and its arguments |
| `blocks` | List of block prefixes this adapter handles (e.g. `["run:myapp"]`) |
| `checks` | List of check names this adapter handles (e.g. `["user-exists"]`) |

### Reporter Fields

| Field | Description |
|-------|-------------|
| `builtin` | Reporter type: `"html"` or `"json"` |
| `outFile` | Output path. For HTML, this is a directory; for JSON, a file path |

### Models Fields

The entire `models` object is optional. When omitted, alloy is enabled
by default.

| Field | Description |
|-------|-------------|
| `builtin` | Model checker: only `"alloy"` is supported |

## Defaults

When fields are omitted from a config file, sensible defaults are applied:

| Field | Default |
|-------|---------|
| `entry` | `specs/index.spec.md` |
| `adapters` | `[]` (empty — built-in shell adapter handles `run:shell`) |
| `models.builtin` | `"alloy"` |
| `reporters` | `[{"builtin":"html","outFile":"specs/report"}, {"builtin":"json","outFile":"specs/report.json"}]` |
| `ignorePrefixes` | `[]` (empty) |
| `trace` | not set (traceability disabled) |
| `toc` | not set (auto-group by directory when subdirectories exist; flat otherwise) |
| `setup` | not set (no pre-run command) |
| `teardown` | not set (no post-run command) |
| `defaultTimeoutMsec` | `30000` (30 seconds) |

An empty config `{}` is valid — all fields are defaulted.

```run:shell
# Verify empty config applies defaults
mkdir -p .tmp-test/defaults-test
printf '# T\n\n- [S](s.spec.md)\n' > .tmp-test/defaults-test/specs/index.spec.md
mkdir -p .tmp-test/defaults-test/specs
printf '# T\n\n- [S](s.spec.md)\n' > .tmp-test/defaults-test/specs/index.spec.md
printf '# S\n\nProse.\n' > .tmp-test/defaults-test/specs/s.spec.md
echo '{}' > .tmp-test/defaults-test/specdown.json
cd .tmp-test/defaults-test && specdown run -dry-run 2>&1 | grep 'spec(s)'
```

specdown runs without a config file when `specs/index.spec.md` exists.

```run:shell
# Verify specdown works with no config file
rm -rf .tmp-test/no-config-test && mkdir -p .tmp-test/no-config-test/specs
printf '# T\n\n- [S](s.spec.md)\n' > .tmp-test/no-config-test/specs/index.spec.md
printf '# S\n\nProse.\n' > .tmp-test/no-config-test/specs/s.spec.md
cd .tmp-test/no-config-test && specdown run -dry-run 2>&1 | grep 'spec(s)'
```

## TOC Grouping

The `toc` field controls how documents are organized in the HTML report sidebar.
Each entry is either a string (standalone document) or a group object with a name
and a list of document paths.

```json
{
  "toc": [
    { "group": "Core", "docs": ["specs/syntax.spec.md", "specs/cli.spec.md"] },
    { "group": "Advanced", "docs": ["specs/alloy.spec.md", "specs/traceability.spec.md"] },
    "specs/overview.spec.md"
  ]
}
```

String entries appear as ungrouped items. Group entries render as collapsible
sections in the sidebar. The current document's group is expanded by default;
others are collapsed.

**Status propagation**: if any document in a group has a failed test case,
the group header shows a red status dot. Expected-fail propagates similarly.

**Type badges**: when a document has a frontmatter `type` field, a small
colored badge appears next to its title in the sidebar.

**Auto-grouping fallback**: documents not listed in `toc` are automatically
grouped by their directory. Documents in the same directory as the entry file
remain ungrouped; documents in subdirectories form groups named after the
directory (e.g., `specs/stories/` becomes "Stories").

When `toc` is omitted entirely, auto-grouping by directory is applied if the
spec tree spans multiple directories. If all documents are in a single
directory, the sidebar renders as a flat list (backward-compatible).

## Validation

The routing model guarantees that no block prefix is handled by more
than one adapter. User adapters have exclusive claims, and the built-in
adapter yields to any user adapter that claims the same prefix.

```alloy:model(routing)
module routing

sig Prefix {}
sig Adapter { handles: set Prefix }
one sig Builtin extends Adapter {}

-- user adapters never overlap
fact exclusiveUserClaims {
  all disj a1, a2: Adapter - Builtin |
    no a1.handles & a2.handles
}

-- builtin yields when a user adapter claims the same prefix
fact builtinYields {
  no p: Prefix |
    p in Builtin.handles and p in (Adapter - Builtin).handles
}

-- every prefix is handled by at most one adapter
assert noConflict {
  all p: Prefix | lone a: Adapter | p in a.handles
}

check noConflict for 6
```

Two adapters with the same name must be rejected.

```run:shell
# Reject duplicate adapter names
mkdir -p .tmp-test
cat <<'CFG' > .tmp-test/dup-adapter.json
{
  "entry": "index.spec.md",
  "adapters": [
    {"name": "a", "command": ["true"], "blocks": ["run:x"]},
    {"name": "a", "command": ["true"], "blocks": ["run:y"]}
  ]
}
CFG
! specdown run -config .tmp-test/dup-adapter.json 2>/dev/null
```

An adapter with an empty name must be rejected.

```run:shell
# Reject adapter with empty name
mkdir -p .tmp-test
printf '{"entry":"i.spec.md","adapters":[{"name":"","command":["true"],"blocks":["run:x"]}]}' > .tmp-test/empty-name.json
! specdown run -config .tmp-test/empty-name.json 2>/dev/null
```

An adapter without a command must be rejected.

```run:shell
# Reject adapter with empty command
printf '{"entry":"i.spec.md","adapters":[{"name":"a","command":[],"blocks":["run:x"]}]}' > .tmp-test/no-cmd.json
! specdown run -config .tmp-test/no-cmd.json 2>/dev/null
```

An adapter must declare at least one block or check.

```run:shell
# Reject adapter with no blocks and no checks
printf '{"entry":"i.spec.md","adapters":[{"name":"a","command":["true"]}]}' > .tmp-test/no-blocks.json
! specdown run -config .tmp-test/no-blocks.json 2>/dev/null
```

Only `"alloy"` is supported as a models builtin. Unknown values are rejected.

```run:shell
# Reject unknown models builtin
printf '{"entry":"i.spec.md","adapters":[],"models":{"builtin":"unknown"}}' > .tmp-test/bad-model.json
! specdown run -config .tmp-test/bad-model.json 2>/dev/null
```

### Relative Paths

All paths in the config are resolved relative to the config file's directory.
This ensures the project works from any checkout location.

```run:shell
# Verify relative entry path resolves from config directory
mkdir -p .tmp-test/relpath/specs
printf '# T\n\n- [S](s.spec.md)\n' > .tmp-test/relpath/specs/index.spec.md
printf '# S\n\nProse.\n' > .tmp-test/relpath/specs/s.spec.md
printf '{"entry":"specs/index.spec.md","adapters":[]}' > .tmp-test/relpath/specdown.json
specdown run -config .tmp-test/relpath/specdown.json -dry-run 2>&1 | grep 'spec(s)'
```
