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

## Init

`specdown init` scaffolds a new project with a config file, entry file, and example spec.

```run:shell -> $initOutput
rm -rf .tmp-test/init-test && mkdir -p .tmp-test/init-test && cd .tmp-test/init-test && specdown init 2>&1
```

```verify:shell
test -f .tmp-test/init-test/specdown.json
test -f .tmp-test/init-test/specs/index.spec.md
test -f .tmp-test/init-test/specs/example.spec.md
```

Running init again in the same directory must fail (no overwrite).

```verify:shell
cd .tmp-test/init-test && ! specdown init 2>/dev/null
```

The generated project must be runnable immediately.

```verify:shell
cd .tmp-test/init-test && specdown run -dry-run 2>&1 | grep -q "spec"
```

## Commands

| Command | Description |
|---------|-------------|
| `specdown init` | Scaffold a new project |
| `specdown run` | Parse, execute, and generate reports in one pass |
| `specdown install skills` | Install Claude Code skills for this project |
| `specdown version` | Print the build version |
| `specdown alloy dump` | Generate Alloy model `.als` files without running adapters |

Every command listed above must appear in the help output.

```verify:shell
help=$(specdown --help 2>&1)
echo "$help" | grep -q "init"
echo "$help" | grep -q "run"
echo "$help" | grep -q "install skills"
echo "$help" | grep -q "version"
echo "$help" | grep -q "alloy dump"
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `specdown.json` | Config file path |
| `-out` | (per config file) | HTML report output path |
| `-filter` | (none) | Heading path substring filter |
| `-jobs` | `1` | Number of spec files to run in parallel |
| `-dry-run` | `false` | Parse and validate only |
| `-show-bindings` | `false` | Print resolved variable bindings for each case |

## Filter

The `-filter` flag runs only cases whose heading path contains the given string.

```run:shell -> $filterOutput
specdown run -config selfspec.json -dry-run -filter "Filter" 2>&1
```

```verify:shell
echo "${filterOutput}" | grep -q "Filter"
```
