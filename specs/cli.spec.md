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

### A Minimal Spec

A well-formed spec document needs only a heading and prose to parse successfully.
Executable blocks and check tables are added as needed.

```run:shell
# Create a minimal spec and verify it parses
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

```run:shell
# Scaffold a new project
rm -rf .tmp-test/init-test && mkdir -p .tmp-test/init-test && cd .tmp-test/init-test && specdown init 2>&1
```

The scaffolded project contains all required files.

```run:shell
$ test -f .tmp-test/init-test/specdown.json && echo yes
yes
$ test -f .tmp-test/init-test/specs/index.spec.md && echo yes
yes
$ test -f .tmp-test/init-test/specs/example.spec.md && echo yes
yes
```

Running init again in the same directory must fail (no overwrite).

```run:shell
# Refuse to overwrite existing project
cd .tmp-test/init-test && ! specdown init 2>/dev/null
```

The generated project must be runnable immediately.

```run:shell
$ cd .tmp-test/init-test && specdown run -dry-run 2>&1 | grep 'spec(s)'
...
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

```run:shell
# Verify all commands appear in help
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

```run:shell
$ specdown run -dry-run -filter "Filter" 2>&1 | head -1
...
```
