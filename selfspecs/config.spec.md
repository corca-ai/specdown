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
      "blocks": ["run:myapp", "verify:myapp"],
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

## Config Fields

| Field | Description |
|-------|-------------|
| `entry` | Path to the entry Markdown file. Its H1 is the report title; links define spec order |
| `adapters` | List of adapters that handle executable blocks and fixtures |
| `reporters` | Output generators. `html` and `json` builtins provided |
| `models` | Alloy model verification. Can be omitted if not used |

## Validation

A config file without `entry` must be rejected.

```verify:shell
mkdir -p .tmp-test
echo '{}' > .tmp-test/bad-config.json
! specdown run -config .tmp-test/bad-config.json 2>/dev/null
```

Two adapters with the same name must be rejected.

```verify:shell
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

```verify:shell
mkdir -p .tmp-test
printf '{"entry":"i.spec.md","adapters":[{"name":"","command":["true"],"blocks":["run:x"]}]}' > .tmp-test/empty-name.json
! specdown run -config .tmp-test/empty-name.json 2>/dev/null
```

An adapter without a command must be rejected.

```verify:shell
printf '{"entry":"i.spec.md","adapters":[{"name":"a","command":[],"blocks":["run:x"]}]}' > .tmp-test/no-cmd.json
! specdown run -config .tmp-test/no-cmd.json 2>/dev/null
```

An adapter must declare at least one block or fixture.

```verify:shell
printf '{"entry":"i.spec.md","adapters":[{"name":"a","command":["true"]}]}' > .tmp-test/no-blocks.json
! specdown run -config .tmp-test/no-blocks.json 2>/dev/null
```

Only `"alloy"` is supported as a models builtin. Unknown values are rejected.

```verify:shell
printf '{"entry":"i.spec.md","adapters":[],"models":{"builtin":"unknown"}}' > .tmp-test/bad-model.json
! specdown run -config .tmp-test/bad-model.json 2>/dev/null
```
