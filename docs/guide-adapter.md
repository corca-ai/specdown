# Writing Adapters

An adapter is a program that actually executes the executable blocks and fixture tables of a spec.
It exchanges NDJSON via stdin/stdout and can be implemented in any language.

## Protocol Flow

```
specdown ──stdin──▸ adapter
specdown ◂─stdout── adapter
```

1. specdown sends a `describe` request
2. The adapter responds with `capabilities`, listing supported blocks and fixtures
3. specdown sends a `setup` request (no response required; may be ignored)
4. specdown sends `runCase` requests in document order
5. The adapter responds to each case with `caseStarted` → `casePassed` or `caseFailed`
6. specdown sends a `teardown` request (no response required; may be ignored)

## Minimal Implementation

```python
#!/usr/bin/env python3
import json, sys

def emit(payload):
    sys.stdout.write(json.dumps(payload) + "\n")
    sys.stdout.flush()

def main():
    state = {}
    for raw in sys.stdin:
        if not raw.strip():
            continue
        req = json.loads(raw)

        if req["type"] == "describe":
            emit({
                "type": "capabilities",
                "blocks": ["run:myapp", "verify:myapp"],
                "fixtures": ["user-exists"],
            })
            continue

        if req["type"] in ("setup", "teardown"):
            continue  # may be ignored

        if req["type"] == "runCase":
            case = req["case"]
            emit({"type": "caseStarted", "id": case["id"], "label": ""})

            try:
                bindings = handle(state, case)
                emit({
                    "type": "casePassed",
                    "id": case["id"],
                    "bindings": bindings,
                })
            except Exception as e:
                emit({
                    "type": "caseFailed",
                    "id": case["id"],
                    "message": str(e),
                    "actual": "",  # concise actual value
                })

def handle(state, case):
    # case["block"]  — "run:myapp", "verify:myapp", etc.
    # case["source"] — block body
    # case["fixture"] — fixture name (for table rows)
    # case["columns"], case["cells"] — table columns and cell values
    # case["bindings"] — variables captured from previous blocks
    # case["captureNames"] — list of variable names to capture
    return []

if __name__ == "__main__":
    main()
```

## Case Types

### Executable Block

`case["kind"]` is `"code"`.

| Field | Description |
|-------|-------------|
| `block` | Info string such as `run:myapp`, `verify:myapp` |
| `source` | Block body (variables already substituted) |
| `bindings` | `[{"name": "x", "value": "1"}, ...]` — for reference |
| `captureNames` | `["userId"]` — variable names to return as results |

Variables in `source` are already substituted. The adapter can execute directly without additional substitution.

If capture is needed, include `[{"name": "userId", "value": "42"}]` in `casePassed.bindings`.

### Fixture Table Row

`case["kind"]` is `"tableRow"`.

| Field | Description |
|-------|-------------|
| `fixture` | Fixture name (`"user-exists"`) |
| `columns` | `["name", "exists"]` |
| `cells` | `["alice", "yes"]` — values with variables already substituted |

## Failure Response

Fill the `actual` field concisely in `caseFailed`.
Since the spec body serves as the expected value in the report, providing only the actual value is sufficient.

```json
{
  "type": "caseFailed",
  "id": {"file": "...", "headingPath": [...], "ordinal": 1},
  "message": "expected column 'done', got 'doing'",
  "actual": "doing",
  "stderr": "optional stderr output"
}
```

If `actual` is present, it is displayed in the report. Otherwise, `message` is displayed.

## Stderr

Both `casePassed` and `caseFailed` can include an optional `stderr` field.
If the adapter captures stderr output during execution, it can be included here and viewed in the report.

## Setup / Teardown

specdown sends a `setup` request before the first `runCase` and a `teardown` request after the last `runCase`.
The adapter may ignore these or use them for test environment initialization and cleanup.
No response is required.

```json
{"type": "setup", "protocol": "specdown-adapter/v1"}
{"type": "teardown", "protocol": "specdown-adapter/v1"}
```

## Timeout

If `timeout` is set in the spec file's frontmatter, specdown waits for each `runCase` response within the specified time.
On timeout, the case is automatically marked as failed. The adapter process itself is not immediately terminated, but it may affect subsequent case execution.

## State Management

The adapter process stays alive during a spec run, so it can maintain process-local state.
For web server testing, know the server URL via environment variables or hardcoding and send HTTP requests.

## Registration

Register the command in `specdown.json`.

```json
{
  "adapters": [{
    "name": "myapp",
    "command": ["python3", "./tools/myapp_adapter.py"],
    "protocol": "specdown-adapter/v1"
  }]
}
```

Any language works as long as it is an executable. Node, Ruby, Go, and shell scripts are all supported.
