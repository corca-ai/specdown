# Adapter Tutorial

This tutorial walks through building a minimal adapter.
For the protocol reference (request/response formats, fields, behavior rules),
see the [Specdown self-spec](../selfspecs/specdown.spec.md#adapter-protocol).

## Minimal Python adapter

```python
#!/usr/bin/env python3
import json, sys

for line in sys.stdin:
    req = json.loads(line)
    if req["type"] in ("setup", "teardown"):
        continue
    if req["type"] == "runCase":
        result = handle(req["case"])
        print(json.dumps({"id": req["id"], **result}))

def handle(case):
    # case["kind"]    — "code" or "tableRow"
    # case["block"]   — "run:myapp", "verify:myapp", etc.
    # case["source"]  — block body (variables already substituted)
    # case["fixture"] — fixture name (for table rows)
    # case["fixtureParams"] — {"key": "value"} from directive (optional)
    # case["columns"], case["cells"] — table data (escapes already resolved)
    # case["bindings"] — variables from previous blocks
    # case["captureNames"] — variable names to return
    try:
        bindings = execute(case)
        return {"type": "passed", "bindings": bindings}
    except Exception as e:
        return {"type": "failed", "message": str(e)}
```

## Minimal shell adapter

```sh
#!/bin/sh
while IFS= read -r line; do
  type=$(echo "$line" | grep -o '"type":"[^"]*"' | head -1 | cut -d'"' -f4)
  case "$type" in
    setup|teardown) ;;
    runCase)
      id=$(echo "$line" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
      # Process the case and emit a response
      echo "{\"id\":${id},\"type\":\"passed\"}"
      ;;
  esac
done
```

## Registration

Declare the adapter and its capabilities in `specdown.json`.

```json
{
  "adapters": [{
    "name": "myapp",
    "command": ["python3", "./tools/myapp_adapter.py"],
    "blocks": ["run:myapp", "verify:myapp"],
    "fixtures": ["user-exists"]
  }]
}
```

Any executable works — Python, Node, Ruby, Go, Rust, shell scripts.

## State management

The adapter process stays alive during a spec run.
It can maintain process-local state (e.g., created resources, session tokens).
For web server testing, connect via HTTP using environment variables or config.

## Structured failure reporting

When a case fails, include `expected` and `actual` for structured diffs
in the CLI output and HTML report.

```json
{
  "id": 1,
  "type": "failed",
  "message": "content mismatch",
  "expected": "alpha\nbeta",
  "actual": "alpha\ngamma",
  "label": "list: empty middle splits"
}
```

The `label` field provides a human-readable row identifier.
If omitted, specdown uses the default `row N` format.
