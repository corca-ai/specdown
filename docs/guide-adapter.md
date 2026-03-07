# Writing Adapters

An adapter is a program that actually executes the executable blocks and fixture tables of a spec.
It exchanges NDJSON via stdin/stdout and can be implemented in any language.

## Protocol Flow

```
specdown ──stdin──▸ adapter
specdown ◂─stdout── adapter
```

1. specdown sends a `setup` request (no response required; may be ignored)
2. specdown sends `runCase` requests in document order, each with an integer `id`
3. The adapter responds to each case with `passed` or `failed`, echoing the `id`
4. specdown sends a `teardown` request (no response required; may be ignored)

Capabilities (which blocks and fixtures the adapter handles) are declared in `specdown.json`, not at runtime.

## Minimal Implementation

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
    # case["block"]  — "run:myapp", "verify:myapp", etc.
    # case["source"] — block body
    # case["fixture"] — fixture name (for table rows)
    # case["fixtureParams"] — {"key": "value"} from fixture directive (optional)
    # case["columns"], case["cells"] — table columns and cell values (escapes already resolved)
    # case["bindings"] — variables captured from previous blocks
    # case["captureNames"] — list of variable names to capture
    try:
        bindings = execute(case)
        return {"type": "passed", "bindings": bindings}
    except Exception as e:
        return {"type": "failed", "message": str(e)}
```

## Request Format

### runCase

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

The integer `id` is a correlation ID assigned by specdown. The adapter must echo it back in the response.

### setup / teardown

```json
{"type": "setup"}
{"type": "teardown"}
```

No response required. The adapter may ignore these or use them for environment initialization and cleanup.

## Response Format

### Passed

```json
{"id": 1, "type": "passed"}
{"id": 1, "type": "passed", "bindings": [{"name": "boardName", "value": "board-1"}]}
```

### Failed

```json
{"id": 1, "type": "failed", "message": "expected 3, got 4"}
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

If capture is needed, include `[{"name": "userId", "value": "42"}]` in `passed.bindings`.

### Fixture Table Row

`case["kind"]` is `"tableRow"`.

| Field | Description |
|-------|-------------|
| `fixture` | Fixture name (`"user-exists"`) |
| `fixtureParams` | `{"user": "alan"}` — optional parameters from the directive |
| `columns` | `["name", "exists"]` |
| `cells` | `["alice", "yes"]` — values with variables already substituted and escapes resolved |

Fixture parameters come from the directive syntax `<!-- fixture:name(key=value) -->`. If no parameters are specified, `fixtureParams` is omitted.

Cell escape sequences (`\n`, `\|`, `\\`) are resolved by specdown before sending. The adapter receives plain values (e.g., a real newline character, not the two-character sequence `\n`).

## Timeout

If `timeout` is set in the spec file's frontmatter, specdown waits for each `runCase` response within the specified time.
On timeout, the case is automatically marked as failed. The adapter process itself is not immediately terminated, but it may affect subsequent case execution.

## State Management

The adapter process stays alive during a spec run, so it can maintain process-local state.
For web server testing, know the server URL via environment variables or hardcoding and send HTTP requests.

## Registration

Declare the adapter command and its capabilities in `specdown.json`.

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

Any language works as long as it is an executable. Node, Ruby, Go, and shell scripts are all supported.
