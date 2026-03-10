---
type: spec
---

# Configuration

Every project needs a `specdown.json`. It tells specdown where specs live,
which [depends::adapters](adapter-protocol.spec.md) to launch, and what
[reporters](report.spec.md) to generate.

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
  ],
  "reporters": [
    { "builtin": "html", "outFile": ".artifacts/specdown/report" },
    { "builtin": "json", "outFile": ".artifacts/specdown/report.json" }
  ],
  "models": { "builtin": "alloy" }
}
```

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
$ specdown run -config .tmp-test/builtin-shell-cfg.json 2>&1 | head -1
PASS 2 spec(s), 1 case(s), 0 alloy check(s)
```

If a user adapter explicitly claims a shell block (e.g., `"blocks": ["run:shell"]`),
the user adapter takes precedence over the built-in.

## Config Fields

| Field | Description |
|-------|-------------|
| `entry` | Path to the entry Markdown file. Starting point for recursive link crawling |
| `adapters` | List of adapters that handle executable blocks and checks |
| `reporters` | Output generators. `html` and `json` builtins provided |
| `models` | Alloy model verification. Can be omitted if not used |
| `ignorePrefixes` | List of code block prefixes to suppress unknown-prefix warnings for |
| `trace` | Trace graph configuration. See [Trace Graph](trace.spec.md) |

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

| Field | Description |
|-------|-------------|
| `builtin` | Model checker: only `"alloy"` is supported |

## Defaults

When fields are omitted from a config file, sensible defaults are applied:
- `entry` defaults to `specs/index.spec.md`
- `models.builtin` defaults to `"alloy"`
- `reporters` defaults to HTML and JSON reporters in `specs/`

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

## Validation

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
