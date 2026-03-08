# Adapter Protocol

The adapter boundary is a process protocol, not an in-process language API.
This allows each project to build adapters with minimal effort in any language.

An adapter is an executable that exchanges NDJSON messages via stdin/stdout.
Any language works as long as it reads JSON from stdin and writes JSON to stdout.

## Protocol Flow

1. specdown launches the adapter process
2. specdown sends a `setup` message (no response required)
3. specdown sends `runCase` messages in document order, each with an integer `id`
4. The adapter responds to each case with `passed` or `failed`, echoing the `id`
5. specdown sends a `teardown` message (no response required)

A single adapter process handles multiple `runCase` requests during one spec run.
The adapter can maintain process-local state across requests.

## Request Format

For executable blocks (`kind: "code"`):

```json
{
  "type": "runCase",
  "id": 1,
  "case": {
    "kind": "code",
    "block": "run:myapp",
    "source": "create-board",
    "captureNames": ["boardName"],
    "bindings": [{"name": "x", "value": "1"}]
  }
}
```

For fixture table rows (`kind: "tableRow"`):

```json
{
  "type": "runCase",
  "id": 2,
  "case": {
    "kind": "tableRow",
    "fixture": "board-exists",
    "fixtureParams": {"user": "alan"},
    "columns": ["board", "exists"],
    "cells": ["board-1", "yes"],
    "bindings": []
  }
}
```

Variables in `source` and `cells` are already substituted.
Cell escape sequences are already resolved.
The adapter can process values directly without additional substitution.

## Response Format

```json
{"id": 1, "type": "passed"}
{"id": 1, "type": "passed", "bindings": [{"name": "boardName", "value": "board-1"}]}
{"id": 1, "type": "failed", "message": "expected 3, got 4"}
{"id": 1, "type": "failed", "message": "mismatch", "expected": "foo", "actual": "bar", "label": "row description"}
```

| Field | Description |
|-------|-------------|
| `id` | Correlation ID, must echo the request `id` |
| `type` | `"passed"` or `"failed"` |
| `message` | Error description (failed only) |
| `expected` | Expected value for structured diff (optional) |
| `actual` | Actual value for structured diff (optional) |
| `label` | Human-readable row identifier, overrides default (optional) |
| `bindings` | Captured variables to pass to subsequent cases (passed only) |
| `steps` | Per-command results for doctest blocks (optional, see below) |

### Doctest Steps

For `doctest:*` blocks, the adapter returns a `steps` array in the response.
Each step records one command and its result:

```json
{
  "id": 1,
  "type": "passed",
  "steps": [
    {"command": "echo hello", "expected": "hello", "actual": "hello", "status": "passed"},
    {"command": "echo world", "expected": "world", "actual": "world", "status": "passed"}
  ]
}
```

| Field | Description |
|-------|-------------|
| `command` | The command that was executed |
| `expected` | Expected output from the spec |
| `actual` | Actual output from execution |
| `status` | `"passed"` or `"failed"` |

On the first mismatch, the block fails. Steps before the failure have status `"passed"`;
the failing step has status `"failed"`.

## Registration

Adapters declare their capabilities in `specdown.json`.
specdown routes each case to the adapter that declared the matching block or fixture.
Capabilities are declared in config, not negotiated at runtime.

```json
{
  "adapters": [{
    "name": "myapp",
    "command": ["python3", "./tools/adapter.py"],
    "blocks": ["run:myapp", "verify:myapp"],
    "fixtures": ["user-exists"]
  }]
}
```

## Adapter Behavior

- A non-zero exit indicates an adapter crash or infrastructure failure, not a case failure
- Only protocol messages are written to stdout; stderr is for diagnostic output
- Adapters may ignore `setup`/`teardown` (no response required)
- Built-in adapters follow the same protocol contract
- The adapter process stays alive during a spec run and can maintain process-local state (e.g., created resources, session tokens)

## Writing an Adapter

Any executable works — Python, Node, Ruby, Go, Rust, shell scripts.
The minimal pattern is: read NDJSON from stdin, skip `setup`/`teardown`,
handle `runCase`, and write the response to stdout.

### Python

```python
#!/usr/bin/env python3
import json, sys

def handle(case):
    # case["kind"]          — "code" or "tableRow"
    # case["block"]         — "run:myapp", "verify:myapp", etc.
    # case["source"]        — block body (variables already substituted)
    # case["fixture"]       — fixture name (for table rows)
    # case["fixtureParams"] — {"key": "value"} from directive (optional)
    # case["columns"], case["cells"] — table data (escapes already resolved)
    # case["bindings"]      — variables from previous blocks
    # case["captureNames"]  — variable names to return
    try:
        bindings = execute(case)
        return {"type": "passed", "bindings": bindings}
    except Exception as e:
        return {"type": "failed", "message": str(e)}

for line in sys.stdin:
    req = json.loads(line)
    if req["type"] in ("setup", "teardown"):
        continue
    if req["type"] == "runCase":
        result = handle(req["case"])
        print(json.dumps({"id": req["id"], **result}))
```

### Shell

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

### Structured Failure Reporting

When a case fails, include `expected` and `actual` for structured diffs
in the CLI output and HTML report. The `label` field provides a
human-readable row identifier; if omitted, specdown uses the default
`row N` format.

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
