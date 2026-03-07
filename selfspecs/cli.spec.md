# CLI

## Getting Started

A specdown workflow has three parts: a spec document that describes behavior,
a configuration file that registers adapters, and the `specdown run` command.

### A Minimal Spec

A well-formed spec document needs only a heading and prose to parse successfully.
Executable blocks and fixture tables are added as needed.

```run:shell
mkdir -p .tmp-test
cat <<'SPEC' > .tmp-test/valid.spec.md
# Valid Spec

Some prose.

## Section

More prose.
SPEC
printf '# T\n\n- [Valid](valid.spec.md)\n' > .tmp-test/index.spec.md
cat <<'CFG' > .tmp-test/valid-cfg.json
{"entry":"index.spec.md","adapters":[]}
CFG
specdown run -config .tmp-test/valid-cfg.json -dry-run 2>&1
```

### Running Specdown

The CLI reports its version when invoked with `version`.

```run:shell -> $version
specdown version
```

```verify:shell
echo "${version}" | grep -qE '^[a-z0-9]'
```

Dry-run mode parses and validates spec files without executing adapters.
This is useful for checking syntax before a full run.

```run:shell -> $dryOutput
specdown run -config selfspec.json -dry-run 2>&1
```

The dry-run output lists discovered specs.

```verify:shell
echo "${dryOutput}" | grep -q "spec"
```

## Commands

| Command | Description |
|---------|-------------|
| `specdown run` | Parse, execute, and generate reports in one pass |
| `specdown version` | Print the build version |
| `specdown alloy dump` | Generate Alloy model `.als` files without running adapters |

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `specdown.json` | Config file path |
| `-out` | (per config file) | HTML report output path |
| `-filter` | (none) | Heading path substring filter |
| `-jobs` | `1` | Number of spec files to run in parallel |
| `-dry-run` | `false` | Parse and validate only |

## Filter

The `-filter` flag runs only cases whose heading path contains the given string.

```run:shell -> $filterOutput
specdown run -config selfspec.json -dry-run -filter "Filter" 2>&1
```

```verify:shell
echo "${filterOutput}" | grep -q "Filter"
```
