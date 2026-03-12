---
type: spec
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
...
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
rm -rf .tmp-test/init-test && mkdir -p .tmp-test/init-test && cd .tmp-test/init-test && specdown init 2>&1
```

```run:shell
$ cd .tmp-test/init-test && specdown run -dry-run 2>&1 | grep 'spec(s)'
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
! specdown trace -config .tmp-test/builtin-shell-cfg.json 2>/dev/null
```

## Filter

The `-filter` flag selects which cases to run. Without a prefix it
matches heading paths by substring, preserving backward compatibility.

```run:shell
$ specdown run -dry-run -filter "Filter" 2>&1 | head -1
...
```

### Type Filter

A `type:` prefix selects cases by kind.

| Filter | Matches |
|--------|---------|
| `type:code` | Code blocks (`run:*`) |
| `type:table` | Check table rows and inline checks |
| `type:expect` | Inline expect assertions |
| `type:alloy` | Alloy model checks only |

```run:shell
# type:alloy keeps only alloy checks; block:shell keeps only code blocks
mkdir -p .tmp-test
printf '%s\n' '# Typed' '' '## Code' '' '```run:shell' 'echo hello' '```' '' '## Model' '' '```alloy:model(tf)' 'module tf' 'sig A {}' 'assert noEmpty { some A }' 'check noEmpty for 3' '```' > .tmp-test/typed-filter.spec.md
printf '# T\n\n- [Typed](typed-filter.spec.md)\n' > .tmp-test/index.spec.md
printf '{"entry":"index.spec.md","adapters":[]}' > .tmp-test/typed-filter-cfg.json
alloy_dry=$(specdown run -config .tmp-test/typed-filter-cfg.json -dry-run -filter "type:alloy" 2>&1)
echo "$alloy_dry" | grep -q 'alloy:'
! echo "$alloy_dry" | grep -q '\[run:shell\]'
block_dry=$(specdown run -config .tmp-test/typed-filter-cfg.json -dry-run -filter "block:shell" 2>&1)
echo "$block_dry" | grep -q '\[run:shell\]'
! echo "$block_dry" | grep -q 'alloy:'
```

### Block and Check Filters

A `block:` prefix matches code blocks by adapter target.
A `check:` prefix matches check table rows by check name.

### Combining Filters

Multiple `-filter` flags narrow the selection. Each flag is additive
within the same prefix but the result is the union of all filters.

```run:shell
# A filter that matches nothing produces zero cases
specdown run -dry-run -filter "NoSuchHeading" 2>&1 | grep -q '0 case'
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
printf 'NOT JSON' > .tmp-test/invalid.json
! specdown run -config .tmp-test/invalid.json 2>/dev/null
```
