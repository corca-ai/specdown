# Configuration

specdown uses a data-only JSON configuration file.
The canonical config must not depend on any specific language runtime.
For v1, a single `specdown.json` is sufficient.

```json
{
  "entry": "specs/index.spec.md",
  "adapters": [
    {
      "name": "myapp",
      "command": ["python3", "./tools/adapter.py"],
      "blocks": ["run:myapp"],
      "fixtures": ["user-exists"]
    }
  ],
  "reporters": [
    { "builtin": "html", "outFile": ".artifacts/specdown/report.html" },
    { "builtin": "json", "outFile": ".artifacts/specdown/report.json" }
  ],
  "models": { "builtin": "alloy" }
}
```

## Entry File

The `entry` field points to a Markdown file whose H1 heading becomes the report title.
The entry file lists spec documents as Markdown links; their order determines the table of contents.

```run:shell
mkdir -p .tmp-test
cat <<'SPEC' > .tmp-test/entry-test.spec.md
# My Feature

Some prose.
SPEC
printf '# My Project Title\n\n- [Feature](entry-test.spec.md)\n' > .tmp-test/entry-index.spec.md
cat <<'CFG' > .tmp-test/entry-test-cfg.json
{"entry":"entry-index.spec.md","adapters":[],"reporters":[{"builtin":"html","outFile":"entry-report.html"}]}
CFG
specdown run -config .tmp-test/entry-test-cfg.json 2>&1 || true
```

The H1 heading from the entry file appears as the report title.

```doctest:shell
$ grep -o '<title>[^<]*</title>' .tmp-test/entry-report.html
<title>My Project Title</title>
```

## Built-in Shell Adapter

The shell adapter is built into specdown. Blocks `run:shell`
and `doctest:shell` work without any adapter configuration.

```run:shell
mkdir -p .tmp-test
printf '# T\n\n- [S](builtin-shell-test.spec.md)\n' > .tmp-test/builtin-shell-index.spec.md
BT=$(printf '\x60\x60\x60')
printf '%s\n' '# Builtin Shell' '' "$BT"'doctest:shell' '$ echo works' 'works' "$BT" > .tmp-test/builtin-shell-test.spec.md
printf '{"entry":"builtin-shell-index.spec.md","adapters":[]}' > .tmp-test/builtin-shell-cfg.json
specdown run -config .tmp-test/builtin-shell-cfg.json 2>&1 || true
```

```doctest:shell
$ grep -o 'PASS' .tmp-test/builtin-shell-cfg.json 2>/dev/null; specdown run -config .tmp-test/builtin-shell-cfg.json 2>&1 | head -1
PASS 1 spec(s), 1 case(s), 0 alloy check(s)
```

If a user adapter explicitly claims a shell block (e.g., `"blocks": ["run:shell"]`),
the user adapter takes precedence over the built-in.

## Config Fields

| Field | Description |
|-------|-------------|
| `entry` | Path to the entry Markdown file. Its H1 is the report title; links define spec order |
| `adapters` | List of adapters that handle executable blocks and fixtures |
| `reporters` | Output generators. `html` and `json` builtins provided |
| `models` | Alloy model verification. Can be omitted if not used |

## Validation

A config file without `entry` must be rejected.

```run:shell
mkdir -p .tmp-test
echo '{}' > .tmp-test/bad-config.json
! specdown run -config .tmp-test/bad-config.json 2>/dev/null
```

Two adapters with the same name must be rejected.

```run:shell
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
mkdir -p .tmp-test
printf '{"entry":"i.spec.md","adapters":[{"name":"","command":["true"],"blocks":["run:x"]}]}' > .tmp-test/empty-name.json
! specdown run -config .tmp-test/empty-name.json 2>/dev/null
```

An adapter without a command must be rejected.

```run:shell
printf '{"entry":"i.spec.md","adapters":[{"name":"a","command":[],"blocks":["run:x"]}]}' > .tmp-test/no-cmd.json
! specdown run -config .tmp-test/no-cmd.json 2>/dev/null
```

An adapter must declare at least one block or fixture.

```run:shell
printf '{"entry":"i.spec.md","adapters":[{"name":"a","command":["true"]}]}' > .tmp-test/no-blocks.json
! specdown run -config .tmp-test/no-blocks.json 2>/dev/null
```

Only `"alloy"` is supported as a models builtin. Unknown values are rejected.

```run:shell
printf '{"entry":"i.spec.md","adapters":[],"models":{"builtin":"unknown"}}' > .tmp-test/bad-model.json
! specdown run -config .tmp-test/bad-model.json 2>/dev/null
```
