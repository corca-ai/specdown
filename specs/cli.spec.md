---
type: spec
workdir: .tmp-test
---

# CLI

The `specdown` command line drives every workflow: scaffolding projects,
running specs, generating reports, and dumping Alloy models.

## Getting Started

A specdown workflow has three parts:
a [depends::spec document](syntax.spec.md) that describes behavior,
a [depends::configuration file](config.spec.md) that registers adapters,
and the `specdown run` command.

```run:shell
$ specdown version
dev
```

Dry-run mode parses and validates spec files without executing adapters.
This is useful for checking syntax before a full run.

```run:shell
$ specdown run -dry-run 2>&1 | grep 'spec(s)'
...
```

## Init

`specdown init` scaffolds a new project with a config file, entry file, and example spec.
The generated project is runnable immediately.

```run:shell
# Scaffold a new project and verify it runs
rm -rf init-test && mkdir -p init-test && cd init-test && specdown init 2>&1
```

```run:shell
$ cd init-test && specdown run -dry-run 2>&1 | grep 'spec(s)'
...
```

## Commands

| Command | Description |
|---------|-------------|
| `specdown init` | Scaffold a new project |
| `specdown run` | Parse, execute, and generate reports in one pass |
| `specdown trace` | Validate trace graph and output results |
| `specdown install skills` | Install Claude Code skills for this project |
| `specdown version` | Print the build version |
| `specdown alloy dump` | Generate Alloy model `.als` files without running adapters |

Every command listed above must appear in the help output.

```run:shell
# Verify all commands appear in help
help=$(specdown --help 2>&1)
echo "$help" | grep -q "init"
echo "$help" | grep -q "run"
echo "$help" | grep -q "trace"
echo "$help" | grep -q "install skills"
echo "$help" | grep -q "version"
echo "$help" | grep -q "alloy dump"
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `specdown.json` | Config file path |
| `-out` | (per config file) | HTML report output path |
| `-filter` | (none) | Case filter (heading substring or `type:`, `block:`, `check:` prefix) |
| `-jobs` | `1` | Number of spec files to run in parallel |
| `-dry-run` | `false` | Parse and validate only |
| `-show-bindings` | `false` | Print resolved variable bindings for each case |
| `-quiet` | `false` | Suppress progress output; show only final summary |
| `-max-failures` | `0` | Stop after N unexpected failures (0 = unlimited) |

### Trace Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `specdown.json` | Config file path |
| `-format` | `json` | Output format: `json`, `dot`, or `matrix` |
| `-strict` | `false` | Suppress output and exit non-zero when validation errors exist |

## Trace

The `specdown trace` command validates the [depends::traceability](traceability.spec.md)
graph configured in `specdown.json` and outputs the result.

Without `-strict`, validation errors are printed to stderr but the graph
is still written to stdout and the command exits successfully.
With `-strict`, any validation error causes a non-zero exit.

Three output formats are supported: `json` (structured graph data),
`dot` (Graphviz), and `matrix` (tabular traceability matrix).

The trace command requires a `trace` key in the config file.

```run:shell
# Verify trace command rejects configs without trace key
! specdown trace -config builtin-shell-cfg.json 2>/dev/null
```

## Filter

The `-filter` flag selects which cases to run. Without a prefix it
matches heading paths by substring, preserving backward compatibility.

```run:shell
# Create a minimal project to test filter
BT=$(printf '\140\140\140')
printf '%s\n' '# F' '' '## Filter Target' '' "\${BT}run:shell" 'true' "\${BT}" '' '## Other' '' "\${BT}run:shell" 'true' "\${BT}" > filter-demo.spec.md
printf '# T\n\n- [F](filter-demo.spec.md)\n' > filter-demo-index.spec.md
printf '{"entry":"filter-demo-index.spec.md","adapters":[]}\n' > filter-demo-cfg.json
```

```run:shell
$ specdown run -config filter-demo-cfg.json -dry-run -filter "Filter Target" 2>&1 | grep 'case(s)'
total: 1 spec(s), 1 case(s)
```

### Type Filter

A `type:` prefix selects cases by kind.

| Filter | Matches |
|--------|---------|
| `type:code` | Code blocks (`run:*`) |
| `type:table` | Check table rows |
| `type:expect` | Inline expect assertions |
| `type:alloy` | Alloy model checks only |

```run:shell
# type:alloy keeps only alloy checks; block:shell keeps only code blocks
printf '%s\n' '# Typed' '' '## Code' '' '```run:shell' 'echo hello' '```' '' '## Model' '' '```alloy:model(tf)' 'module tf' 'sig A {}' 'assert noEmpty { some A }' 'check noEmpty for 3' '```' > typed-filter.spec.md
printf '# T\n\n- [Typed](typed-filter.spec.md)\n' > index.spec.md
printf '{"entry":"index.spec.md","adapters":[]}' > typed-filter-cfg.json
alloy_dry=$(specdown run -config typed-filter-cfg.json -dry-run -filter "type:alloy" 2>&1)
echo "$alloy_dry" | grep -q 'alloy:'
! echo "$alloy_dry" | grep -q '\[run:shell\]'
block_dry=$(specdown run -config typed-filter-cfg.json -dry-run -filter "block:shell" 2>&1)
echo "$block_dry" | grep -q '\[run:shell\]'
! echo "$block_dry" | grep -q 'alloy:'
```

### Block and Check Filters

A `block:` prefix matches code blocks by adapter target.
A `check:` prefix matches check table rows by check name.

### Combining Filters

A single `-filter` flag selects cases. Only one filter can be active
at a time — if `-filter` is passed multiple times, the last value wins.

```run:shell
# A filter that matches nothing produces zero cases
specdown run -config typed-filter-cfg.json -dry-run -filter "NoSuchHeading" 2>&1 | grep -q '0 case'
```

## Install Skills

`specdown install skills` creates an `.agents/skills/specdown/` directory
with a `SKILL.md` and all reference specs so that Claude Code can write,
run, and fix specs without leaving the editor. A `.claude/skills` symlink
pointing to `.agents/skills` is created for Claude Code compatibility.

```run:shell
# Install skills into a fresh directory and list created files
rm -rf skill-install && mkdir -p skill-install
cd skill-install && specdown install skills 2>/dev/null
```

The installed files are the skill definition plus one reference per spec:

```run:shell
$ ls skill-install/.agents/skills/specdown/ | LC_ALL=C sort
SKILL.md
adapter-protocol.md
alloy.md
best-practices.md
cli.md
config.md
internals.md
overview.md
report.md
syntax.md
traceability.md
validation.md
workflow-adopt.md
workflow-evolve.md
workflow-new-project.md
```

The `.claude/skills` path is a symlink to `.agents/skills`:

```run:shell
$ readlink skill-install/.claude/skills
../.agents/skills
```

Running the command again without `--overwrite` is rejected:

```run:shell
$ cd skill-install && specdown install skills 2>&1 | grep -q 'already exists' && echo "blocked"
blocked
```

With `--overwrite`, existing files are replaced:

```run:shell
# Overwrite succeeds
cd skill-install && specdown install skills --overwrite 2>/dev/null
```

Existing `.claude/skills` directories are migrated automatically:

```run:shell
# Create a legacy .claude/skills directory
rm -rf skill-migrate && mkdir -p skill-migrate/.claude/skills/specdown
echo "old" > skill-migrate/.claude/skills/specdown/SKILL.md
cd skill-migrate && specdown install skills --overwrite 2>/dev/null
# Verify migration: .agents/skills exists and .claude/skills is a symlink
test -d .agents/skills/specdown && readlink .claude/skills && echo "migrated"
```

```run:shell
$ cd skill-migrate && test -d .agents/skills/specdown && readlink .claude/skills && echo "migrated"
../.agents/skills
migrated
```

## Error Messages

The CLI reports clear errors for common mistakes.

### Missing Config File

When `-config` points to a non-existent file, the error is immediate.

```run:shell
! specdown run -config nonexistent.json 2>/dev/null
```

### Invalid JSON Config

Malformed JSON in the config file is reported as a parse error.

```run:shell
# Reject syntactically invalid JSON
printf 'NOT JSON' > invalid.json
! specdown run -config invalid.json 2>/dev/null
```
