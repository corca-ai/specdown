# Writing Adapters

Adapter는 spec의 실행 블록과 fixture 표를 실제로 실행하는 프로그램이다.
stdin/stdout으로 NDJSON을 주고받으며, 어떤 언어로든 구현할 수 있다.

## 프로토콜 흐름

```
specdown ──stdin──▸ adapter
specdown ◂─stdout── adapter
```

1. specdown이 `describe` 요청을 보낸다
2. adapter가 `capabilities`로 지원하는 블록과 fixture를 응답한다
3. specdown이 `setup` 요청을 보낸다 (응답 불필요, 무시해도 됨)
4. specdown이 `runCase`를 문서 순서대로 보낸다
5. adapter가 각 case에 `caseStarted` → `casePassed` 또는 `caseFailed`로 응답한다
6. specdown이 `teardown` 요청을 보낸다 (응답 불필요, 무시해도 됨)

## 최소 구현

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
            continue  # 무시해도 됨

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
                    "actual": "",  # 간결한 실제 값
                })

def handle(state, case):
    # case["block"]  — "run:myapp", "verify:myapp" 등
    # case["source"] — 블록 본문
    # case["fixture"] — fixture 이름 (표 행인 경우)
    # case["columns"], case["cells"] — 표 컬럼과 셀 값
    # case["bindings"] — 이전 블록에서 캡처된 변수
    # case["captureNames"] — 캡처할 변수 이름 목록
    return []

if __name__ == "__main__":
    main()
```

## Case 종류

### Executable Block

`case["kind"]`가 `"code"`다.

| 필드 | 설명 |
|------|------|
| `block` | `run:myapp`, `verify:myapp` 등 info string |
| `source` | 블록 본문 (변수 치환 완료) |
| `bindings` | `[{"name": "x", "value": "1"}, ...]` — 참고용 |
| `captureNames` | `["userId"]` — 결과로 돌려줄 변수 이름 |

`source`에는 `${변수}`가 이미 치환된 값이 들어온다. adapter는 추가 치환 없이 바로 실행하면 된다.

캡처가 필요하면 `casePassed.bindings`에 `[{"name": "userId", "value": "42"}]`를 넣는다.

### Fixture Table Row

`case["kind"]`가 `"tableRow"`다.

| 필드 | 설명 |
|------|------|
| `fixture` | fixture 이름 (`"user-exists"`) |
| `columns` | `["name", "exists"]` |
| `cells` | `["alice", "yes"]` — 변수 치환 완료된 값 |

## 실패 응답

`caseFailed`에는 `actual` 필드를 간결하게 채운다.
리포트에서 spec 본문이 expected 역할을 하므로, 실제 값만 있으면 충분하다.

```json
{
  "type": "caseFailed",
  "id": {"file": "...", "headingPath": [...], "ordinal": 1},
  "message": "expected column 'done', got 'doing'",
  "actual": "doing",
  "stderr": "optional stderr output"
}
```

`actual`이 있으면 리포트에 표시된다. 없으면 `message`가 표시된다.

## Stderr

`casePassed`와 `caseFailed` 모두 선택적 `stderr` 필드를 포함할 수 있다.
adapter가 실행 중 캡처한 stderr 출력을 여기에 넣으면 리포트에서 확인할 수 있다.

## Setup / Teardown

specdown은 첫 `runCase` 전에 `setup`, 마지막 `runCase` 후에 `teardown` 요청을 보낸다.
adapter는 이를 무시해도 되고, 필요하면 테스트 환경 초기화나 정리에 활용할 수 있다.
응답은 필요 없다.

```json
{"type": "setup", "protocol": "specdown-adapter/v1"}
{"type": "teardown", "protocol": "specdown-adapter/v1"}
```

## 타임아웃

spec 파일의 frontmatter에 `timeout`이 설정되어 있으면, specdown은 각 `runCase`에 대해 응답을 지정 시간 내에 기다린다.
시간 초과 시 해당 case는 자동으로 실패 처리된다. adapter 프로세스 자체는 즉시 종료되지 않지만, 이후 case 실행에 영향을 줄 수 있다.

## 상태 관리

adapter 프로세스는 한 spec run 동안 살아있으므로 process-local state를 유지할 수 있다.
웹서버 테스트라면 서버 URL을 환경변수나 하드코딩으로 알고, HTTP 요청을 보내면 된다.

## 등록

`specdown.json`에 command를 등록한다.

```json
{
  "adapters": [{
    "name": "myapp",
    "command": ["python3", "./tools/myapp_adapter.py"],
    "protocol": "specdown-adapter/v1"
  }]
}
```

실행 파일이면 어떤 언어든 상관없다. Node, Ruby, Go, shell script 모두 가능하다.
