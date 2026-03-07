# Adapter Protocol

An adapter is an executable that exchanges NDJSON messages via stdin/stdout.
Any language works as long as it reads JSON from stdin and writes JSON to stdout.

## Protocol flow

1. specdown sends a `setup` message (no response required)
2. specdown sends `runCase` messages in document order, each with an integer `id`
3. The adapter responds to each case with `passed` or `failed`, echoing the `id`
4. specdown sends a `teardown` message (no response required)

## Request format

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

## Response format

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

## Registration

Adapters declare their capabilities in `specdown.json`.
specdown routes each case to the adapter that declared the matching block or fixture.

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

## Adapter behavior

- A single adapter process handles multiple `runCase` requests during one spec run
- The adapter can maintain process-local state across requests
- A non-zero exit indicates infrastructure failure, not a case failure
- stderr is used for diagnostic output; only protocol messages go to stdout
