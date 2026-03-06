# Configuration & Running

## Configuration File

Create a `specdown.json` in the project root.

```json
{
  "include": ["specs/**/*.spec.md"],
  "adapters": [
    {
      "name": "myapp",
      "command": ["python3", "./tools/myapp_adapter.py"],
      "protocol": "specdown-adapter/v1"
    }
  ],
  "reporters": [
    {
      "builtin": "html",
      "outFile": ".artifacts/specdown/report.html"
    },
    {
      "builtin": "json",
      "outFile": ".artifacts/specdown/report.json"
    }
  ],
  "models": {
    "builtin": "alloy"
  }
}
```

| Field | Description |
|-------|-------------|
| `include` | Glob pattern for spec files |
| `adapters` | List of adapters that handle executable blocks and fixtures |
| `reporters` | Output generators. `html` and `json` builtins provided |
| `models` | Alloy model verification. Can be omitted if not used |

## Running

```sh
specdown run
specdown run -out report.html       # specify report path directly
specdown run -config other.json     # use a different config file
specdown run -filter "board create" # run only cases whose heading path contains the string
specdown run -jobs 4                # run 4 spec files in parallel
specdown run -dry-run               # parse and validate only, no adapter execution
```

Parses all spec files, runs adapters, and generates reports.
On failure, prints detailed information for each failed item to stderr.

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `specdown.json` | Config file path |
| `-out` | (per config file) | HTML report output path |
| `-filter` | (none) | Heading path substring filter |
| `-jobs` | `1` | Number of spec files to run in parallel |
| `-dry-run` | `false` | Parse and validate only |

## Version Check

```sh
specdown version
specdown --version
```

## Alloy Model Dump

Generates only Alloy model `.als` files without running adapters.

```sh
specdown alloy dump
specdown alloy dump -config other.json
```

## Artifacts

| File | Description |
|------|-------------|
| `.artifacts/specdown/report.html` | Executed specification HTML report |
| `.artifacts/specdown/report.json` | Machine-readable results |
| `.artifacts/specdown/models/*.als` | Combined Alloy models |

## Project Layout Example

```
my-project/
├── specdown.json
├── specs/
│   ├── auth.spec.md
│   └── billing.spec.md
├── tools/
│   └── myapp_adapter.py
└── .artifacts/
    └── specdown/
        └── report.html      ← auto-generated, excluded from version control
```

Add `.artifacts/` to `.gitignore`.
