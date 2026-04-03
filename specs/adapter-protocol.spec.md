---
type: spec
workdir: .tmp-test
---

# Adapter Protocol

Adapters are how specs talk to real code.
The boundary is a process protocol, not an in-process language API —
any executable that reads NDJSON from stdin and writes NDJSON to stdout is an adapter.

Adapters execute the [depends::executable blocks and check tables](syntax.spec.md) defined in spec documents.
This allows each project to build adapters with minimal effort in any language.

## Protocol Flow

1. specdown launches the adapter process
2. specdown sends `exec` or `assert` messages in document order, each with an integer `id` starting from 1 and incrementing sequentially within the session
3. The adapter responds to each message, echoing the exact `id`
4. When the spec run finishes, specdown closes stdin and waits for the process to exit

Sessions are scoped per-document. Each adapter session is started on first
use and closed after all cases in that document complete. With `--jobs N`,
each document gets its own independent sessions.

A single adapter process handles multiple requests during one spec run.
The adapter can maintain process-local state across requests.
Use this to cache data: for example, when a check table has many rows
that all query the same endpoint, fetch once on the first row
and reuse the cached response for subsequent rows.

## Exec Request

For executable blocks:

```json
{
  "type": "exec",
  "id": 1,
  "source": "create-board"
}
```

Variables in `source` are already substituted by the engine.
The adapter executes the source and returns the result.

## Exec Response

An exec response must contain exactly one of `"output"` or `"error"` keys.
Key presence determines success or failure — not the value.

```json
{"id": 1, "output": "board-1"}
{"id": 1, "output": ""}
{"id": 1, "error": "command not found"}
```

| Key | Description |
|-----|-------------|
| `id` | Correlation ID, must echo the request `id` |
| `output` | Present on success. Can be any JSON value (string, object, null, etc.) |
| `error` | Present on failure. Error message string |

The engine handles variable capture: if the block has `-> $var`, the engine
extracts the output value. For string output, lines are split and mapped to
capture names in order. For structured (non-string) output, the value is
stored as-is and accessible via dot-path syntax (`${result.field}`).

## Assert Request

For check table rows and check calls:

```json
{
  "type": "assert",
  "id": 2,
  "check": "board-exists",
  "checkParams": {"user": "alan"},
  "columns": ["board", "exists"],
  "cells": ["board-1", "yes"]
}
```

Variables in `cells` are already substituted.
Cell escape sequences are already resolved.
All cell values are strings after substitution and unescaping.

## Assert Response

```json
{"id": 2, "type": "passed"}
{"id": 2, "type": "failed", "message": "expected 3, got 4"}
{"id": 2, "type": "failed", "message": "mismatch", "expected": "foo", "actual": "bar", "label": "row description"}
```

| Field | Description |
|-------|-------------|
| `id` | Correlation ID, must echo the request `id` |
| `type` | `"passed"` or `"failed"` |
| `message` | Error description (failed only) |
| `expected` | Expected value for structured diff (optional) |
| `actual` | Actual value for structured diff (optional) |
| `label` | Human-readable row identifier, overrides default `row N` format (optional) |

## Reserved Fields

Both exec and assert responses may include an optional `"code"` field
(string) for structured error classification. The engine currently
ignores this field but it is reserved for future use. Adapter authors
should not use `"code"` for other purposes.

## Registration

Adapters declare their capabilities in `specdown.json`.
specdown routes each case to the adapter that declared the matching block or check.
Capabilities are declared in config, not negotiated at runtime.

```json
{
  "adapters": [{
    "name": "myapp",
    "command": ["python3", "./tools/adapter.py"],
    "blocks": ["run:myapp"],
    "checks": ["user-exists"]
  }]
}
```

## Adapter Behavior

- A non-zero exit indicates an adapter crash or infrastructure failure, not a case failure
- Only protocol messages are written to stdout; stderr is for diagnostic output
- Built-in adapters follow the same protocol contract (see below)

## Built-in Shell Adapter

The shell adapter is one of two built-in adapters (the other is the
[jq check](config.spec.md#built-in-jq-check)). It handles `run:shell`
blocks without any adapter configuration.

### Execution Model

The built-in shell adapter runs in-process rather than as a spawned subprocess.
It still follows the same NDJSON protocol contract — the difference is
transparent to spec authors.

All commands are executed via `sh -c`.

### Block Behaviors

**`run:shell`** — Executes the block source as a shell command. A non-zero exit
code returns an error response. If capture names are specified (`-> $var`),
stdout lines are split and bound to variables in order by the engine.

Blocks whose content starts with `$ ` lines are auto-detected as doctest-style.
The engine sends individual `exec` requests for each command and compares
output against expected values inline.

### Override Precedence

If a user adapter explicitly claims a shell block (e.g., `"blocks": ["run:shell"]`),
the user adapter takes precedence. The built-in only registers blocks that no
user adapter has claimed.

## Writing an Adapter

Any executable works — Python, Node, Ruby, Go, Rust, shell scripts.
The minimal pattern is: read NDJSON from stdin, handle `exec` and `assert`
messages, and write the response to stdout.

### Python

```python
#!/usr/bin/env python3
import json, sys

def handle_exec(source):
    try:
        output = execute(source)
        return {"output": output}
    except Exception as e:
        return {"error": str(e)}

def handle_assert(check, params, columns, cells):
    try:
        run_check(check, params, columns, cells)
        return {"type": "passed"}
    except Exception as e:
        return {"type": "failed", "message": str(e)}

for line in sys.stdin:
    req = json.loads(line)
    if req["type"] == "exec":
        result = handle_exec(req["source"])
        print(json.dumps({"id": req["id"], **result}))
    elif req["type"] == "assert":
        result = handle_assert(
            req["check"], req.get("checkParams", {}),
            req.get("columns", []), req.get("cells", []))
        print(json.dumps({"id": req["id"], **result}))
```

### Shell

```sh
#!/bin/sh
while IFS= read -r line; do
  type=$(echo "$line" | grep -o '"type":"[^"]*"' | head -1 | cut -d'"' -f4)
  id=$(echo "$line" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
  case "$type" in
    exec)
      echo "{\"id\":${id},\"output\":\"ok\"}"
      ;;
    assert)
      echo "{\"id\":${id},\"type\":\"passed\"}"
      ;;
  esac
done
```

### End-to-End Example

Here is a minimal adapter that actually runs. It echoes the source back as output
for `exec` requests and passes all `assert` requests.

```run:shell
# Create a minimal echo adapter
cat <<'ADAPTER' > echo-adapter.sh
#!/bin/sh
while IFS= read -r line; do
  type=$(printf '%s' "$line" | grep -o '"type":"[^"]*"' | head -1 | cut -d'"' -f4)
  id=$(printf '%s' "$line" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
  case "$type" in
    exec) printf '{"id":%s,"output":"echo-ok"}\n' "$id" ;;
    assert) printf '{"id":%s,"type":"passed"}\n' "$id" ;;
  esac
done
ADAPTER
chmod +x echo-adapter.sh
```

Wire the adapter to a spec and run it:

```run:shell
# Run a spec through the echo adapter
BT=$(printf '\140\140\140')
printf '%s\n' '# E2E' '' "\${BT}run:echo" 'some source' "\${BT}" > e2e.spec.md
printf '# T\n\n- [E2E](e2e.spec.md)\n' > index.spec.md
printf '{"entry":"index.spec.md","adapters":[{"name":"echo","command":["sh","./echo-adapter.sh"],"blocks":["run:echo"]}]}' > e2e-cfg.json
```

```run:shell
$ specdown run -config e2e-cfg.json 2>&1 | grep 'spec(s)' | sed 's/ in [0-9]*ms//'
PASS 2 spec(s), 1 case(s)
```

### Structured Failure Reporting

When a check fails, include `expected` and `actual` for structured diffs
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

## Timeouts

Every adapter request is subject to a time limit. The timeout is
determined by (in priority order):

1. Per-document `timeout` in frontmatter (milliseconds)
2. `defaultTimeoutMsec` in `specdown.json` (default: `30000`)

A timeout value of `0` disables the time limit — the engine waits
indefinitely for the adapter to respond.

If the adapter does not respond within the timeout:

- **Exec requests**: the engine synthesizes an error response with the message `timeout after Nms (exec: "source")` where *source* is the first 80 characters of the exec source
- **Assert requests**: the engine synthesizes a failed response with the message `timeout after Nms (assert: check "name")` where *name* is the check name

The adapter process is not killed — only the pending request is abandoned.

## Response Size Limit

Adapter responses are limited to 1 MB per line. If a single response
line exceeds this limit, the engine treats it as a read error.

## Error Handling

### Adapter Crash (Non-Zero Exit)

A non-zero exit code from the adapter process signals an infrastructure
failure, not a case failure. specdown reports it as an error, distinct
from a test failure.

```run:shell
# Create an adapter that crashes mid-session
cat <<'ADAPTER' > crash-adapter.sh
#!/bin/sh
read -r line
id=$(printf '%s' "$line" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
printf '{"id":%s,"output":"ok"}\n' "$id"
exit 1
ADAPTER
chmod +x crash-adapter.sh
```

```run:shell
# Wire the crashing adapter to a two-case spec
BT=$(printf '\140\140\140')
printf '%s\n' '# Crash' '' "\${BT}run:boom" 'step1' "\${BT}" '' "\${BT}run:boom" 'step2' "\${BT}" > crash.spec.md
printf '# T\n\n- [Crash](crash.spec.md)\n' > index.spec.md
printf '{"entry":"index.spec.md","adapters":[{"name":"boom","command":["sh","./crash-adapter.sh"],"blocks":["run:boom"]}]}' > crash-cfg.json
! specdown run -config crash-cfg.json 2>/dev/null
```

### Malformed Adapter Response

If the adapter writes invalid JSON, specdown treats it as an error.

```run:shell
# Create an adapter that writes garbage
printf '#!/bin/sh\necho "NOT JSON"\n' > bad-adapter.sh
chmod +x bad-adapter.sh
```

```run:shell
# Run a spec against the malformed adapter
BT=$(printf '\140\140\140')
printf '%s\n' '# Bad' '' "\${BT}run:bad" 'hello' "\${BT}" > bad.spec.md
printf '# T\n\n- [Bad](bad.spec.md)\n' > index.spec.md
printf '{"entry":"index.spec.md","adapters":[{"name":"bad","command":["sh","./bad-adapter.sh"],"blocks":["run:bad"]}]}' > bad-cfg.json
! specdown run -config bad-cfg.json 2>/dev/null
```
