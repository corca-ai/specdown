# Executable Specification (`specdown`)

프로젝트 독립형 executable specification 시스템 설계 문서.
코어 설계는 특정 제품에 종속되지 않는다.


## 문서 상태

- 대상 독자: 독립된 개발팀
- 목적: reusable core + adapter 기반 제품으로 구현 가능할 만큼 요구사항과 경계를 확정
- 결과물: `specdown-core`, `specdown-cli`, `specdown-adapter-protocol`, `specdown-reporter-html`, `specdown-alloy`, reference adapters


## 문제

설계 문서와 테스트 코드가 분리되면 다음 문제가 반복된다.

- 설계 문서에 적힌 속성이 실제로 검증되는지 추적하기 어렵다
- 테스트는 동작을 검증하지만 설계 의도와 이유를 충분히 설명하지 않는다
- 보안 속성이나 상태 공간 성질은 예시 기반 테스트만으로는 충분히 다루기 어렵다
- 문서, 테스트, 형식 모델이 따로 진화하면서 정합성이 사람의 기억에 의존하게 된다


## 목표

`specdown`의 목표는 다음 네 가지다.

1. 하나의 Markdown 문서가 읽을 수 있는 명세서이면서 실행 가능한 테스트가 된다.
2. 같은 문서 안에 Alloy 모델을 literate style로 엮어 넣고 형식 검증까지 연결한다.
3. 표 기반 명세(FIT 스타일)를 first-class 기능으로 제공한다.
4. 실행 결과를 HTML로 렌더링해 문서 자체를 green/red 상태가 드러나는 실행 리포트로 본다.


## 비목표

다음은 v1 범위에서 제외한다.

- 모든 테스트를 Markdown DSL로 대체하는 것
- 구현이 형식 모델과 완전히 동치임을 자동 증명하는 것
- Playwright, Vitest, Jest, Bun test 중 하나를 코어에 내장하는 것
- DOM selector, shell transcript, editor action 같은 프로젝트 특화 DSL을 코어에 포함하는 것
- 다중 저장소/다중 패키지 모델 import를 완전 자동화하는 것


## 핵심 결정

이 문서에서 다음 사항은 확정한다.

1. 문서 포맷은 Markdown-first다. 산문은 보존되고, 실행 블록만 구조적으로 해석한다.
2. 코어는 테스트 프레임워크나 제품 로직을 모른다. 모든 실행 의미는 adapter가 제공한다.
3. Alloy는 문서 임베딩 블록을 중심으로 지원한다.
4. 같은 `alloy:model(name)`를 여러 번 쓰면 문서 순서대로 하나의 logical model로 결합한다.
5. HTML 리포트는 부가 기능이 아니라 1급 산출물이다.
6. 표 기반 명세는 core 문법이고, 각 표의 실행 의미는 fixture adapter가 정의한다.
7. adapter 확장 경계는 language-neutral한 process protocol을 기본값으로 한다.


## 제품 구성

권장 패키지 구성은 다음과 같다.

```text
packages/
├── specdown-core/              # parser, AST, planning, event model
├── specdown-cli/               # run 진입점
├── specdown-adapter-protocol/  # adapter process contract + JSON schema
├── specdown-reporter-html/     # static HTML report renderer
├── specdown-alloy/             # embedded model extraction + Alloy runner
├── specdown-adapter-shell/     # optional high-reuse builtin adapter
└── specdown-adapter-vitest/    # optional convenience adapter
```

`specdown-core`는 `vitest`, `playwright`, `svelte`, DOM selector에 의존하면 안 된다.


## 아키텍처

두 개의 파이프라인이 하나의 문서에서 갈라진다.

```text
Spec Document (.spec.md)
    │
    ├── Spec Core
    │     ├── heading / prose / block / table 파싱
    │     ├── 변수 스코프 계산
    │     ├── executable unit ID 부여
    │     └── embedded Alloy model 추출
    │
    ├── Runtime Adapter
    │     └── 테스트 실행 + Spec Event 방출
    │
    ├── Reporter Adapter
    │     └── HTML / JSON / CI artifact 생성
    │
    └── Alloy Runner
          └── model check + Spec Event 방출
```

핵심 원칙은 simple하다.

- Core는 문서를 구조화하고 실행 계획을 만든다
- Runtime adapter는 각 블록/표를 실제 테스트나 명령으로 바꾼다
- Reporter adapter는 실행 이벤트를 사람이 읽을 수 있는 결과물로 바꾼다
- Alloy runner는 형식 검증 결과를 같은 이벤트 모델로 올린다
- Adapter는 기본적으로 out-of-process command로 연결된다


## 코어와 어댑터 경계

### Core 책임

- Markdown 파싱
- heading hierarchy를 suite hierarchy로 변환
- code block, directive, table 추출
- 변수 바인딩과 스코프 계산
- `SpecId` 생성
- embedded Alloy fragment 결합
- runtime-independent execution plan 생성
- 공통 event schema 정의

### Adapter 책임

- `run:*`, `verify:*`, `test:*` 같은 블록 의미 해석
- fixture table의 컬럼 의미 해석
- shell, browser, API, editor, sandbox 같은 외부 실행 환경 접속
- 테스트 프레임워크 통합
- HTML/JSON/JUnit 같은 결과 렌더링
- process protocol로 core와 통신

### Core가 몰라야 하는 것

- Vitest의 `describe/test` API
- Playwright page object
- 특정 제품의 filesystem layout
- 특정 제품의 command vocabulary
- adapter 구현 언어와 런타임


## 공용 프로토콜

adapter 경계는 in-process language API가 아니라 process protocol이어야 한다.
그래야 각 프로젝트가 Go, Python, Rust, Node, Ruby 등 원하는 언어로 최소 노력으로 adapter를 만들 수 있다.

기본 transport는 다음으로 고정한다.

- adapter는 실행 가능한 command다
- `specdown-cli`가 adapter process를 실행한다
- stdin/stdout으로 NDJSON 메시지를 주고받는다
- stdout에는 protocol message만 쓴다
- 하나의 adapter process는 한 spec run 동안 여러 `runCase` 요청을 순서대로 처리할 수 있다
- `specdown-cli`는 첫 `runCase` 전에 `setup`을, 마지막 `runCase` 후에 `teardown`을 보낸다
- adapter는 `setup`/`teardown`을 무시해도 된다 (응답 불필요)
- non-zero exit는 case failure가 아니라 adapter crash 또는 infrastructure failure로 본다

전송되는 payload는 JSON-serializable shape여야 한다.

```ts
export type SpecId = {
  file: string;
  headingPath: string[];
  ordinal: number;
};

export type CodeBlockNode = {
  kind: "code";
  info: string;
  source: string;
  id: SpecId;
};

export type TableNode = {
  kind: "table";
  fixture: string;
  context: string | null;
  columns: string[];
  rows: string[][];
  id: SpecId;
};

export type Binding = {
  name: string;
  value: string;
};

export type AdapterRequest =
  | { type: "describe"; protocol: "specdown-adapter/v1" }
  | { type: "setup"; protocol: "specdown-adapter/v1" }
  | { type: "teardown"; protocol: "specdown-adapter/v1" }
  | {
      type: "runCase";
      protocol: "specdown-adapter/v1";
      case: {
        id: SpecId;
        kind: "code" | "tableRow";
        block?: string;
        source?: string;
        fixture?: string;
        columns?: string[];
        cells?: string[];
        captureNames?: string[];
        bindings?: Binding[];
      };
    };

export type AdapterResponse =
  | { type: "capabilities"; blocks: string[]; fixtures: string[] }
  | { type: "caseStarted"; id: SpecId; label: string }
  | { type: "casePassed"; id: SpecId; durationMs?: number; bindings?: Binding[]; stderr?: string }
  | { type: "caseFailed"; id: SpecId; message: string; expected?: string; actual?: string; details?: string; stderr?: string }
  | { type: "modelCheckPassed"; model: string; assertion: string }
  | { type: "modelCheckFailed"; model: string; assertion: string; counterexamplePath?: string };
```

핵심 규칙:

- core는 `CodeBlockNode`, `TableNode`, `SpecId`, event schema만 고정한다
- adapter는 `describe`로 자신이 지원하는 block info와 fixture name을 광고한다
- `specdown-cli`는 그 광고를 기준으로 어떤 adapter가 어떤 case를 처리할지 결정한다
- 실행 중에는 문서 순서를 유지한 채 `runCase`를 순서대로 보낸다
- adapter는 process-local state를 유지할 수 있고, `casePassed.bindings`로 값을 core에 돌려줄 수 있다
- adapter failure는 가능하면 `message`뿐 아니라 `expected`와 `actual`도 구조화해서 보낸다
- built-in adapter가 있더라도 같은 protocol contract를 따라야 한다
- 언어별 helper SDK는 optional convenience일 뿐, architecture의 핵심이 아니다


## 설정 방식

구현팀은 data-only 설정 파일을 기본값으로 채택한다.
canonical config는 특정 언어 runtime에 종속되면 안 된다.

예시:

```json
{
  "include": ["specs/**/*.spec.md"],
  "adapters": [
    {
      "name": "project",
      "command": ["python3", "./tools/specdown_adapter.py"],
      "protocol": "specdown-adapter/v1"
    }
  ],
  "reporters": [
    {
      "builtin": "html",
      "outFile": ".artifacts/specdown/report.html"
    }
  ],
  "models": {
    "builtin": "alloy"
  }
}
```

언어별 helper가 이 파일을 생성해 줄 수는 있지만, 정본 format은 data-only여야 한다.
v1에서는 `specdown.json` 하나면 충분하다.


## 문서 문법

### 구조 매핑

Heading hierarchy는 테스트 suite 계층으로 변환된다.

| Markdown | 의미 |
|----------|------|
| `#`, `##`, `###` | suite hierarchy |
| 일반 산문 | 문서 본문, 실행 대상 아님 |
| fenced code block | 실행 블록 또는 모델 블록 |
| HTML comment directive | setup, teardown, fixture, alloy reference 등 메타 지시자 |
| Markdown table | fixture directive와 결합될 때 실행 데이터 |

### 지원 블록

코어는 다음 규칙만 안다.

| 표기 | core 의미 | 실행 의미 제공 주체 |
|------|----------|--------------------|
| `run:<target>` | side-effecting executable block | block adapter |
| `verify:<target>` | assertion-bearing executable block | block adapter |
| `expect` | assertion block | block adapter 또는 core helper |
| `test:<name>` | named high-level test DSL | block adapter |
| `alloy:model(name)` | embedded Alloy fragment | core + Alloy runner |

`<target>`과 `<name>`의 실제 의미는 core가 아니라 adapter가 정한다.

### 변수

문서 안에서 동적 값을 연결하기 위해 변수 바인딩을 지원한다.

예시:

````markdown
```run:shell -> $channelId
create-channel random
```

```expect
${channelId} matches /^ch-/
```
````

규칙:

- 상위 섹션 변수는 하위 섹션에서 읽을 수 있다
- 같은 깊이의 형제 섹션끼리도 공유 가능하다 (문서 순서 기준, 앞에서 캡처한 값만)
- unresolved variable은 compile-time error다
- `\${...}`로 이스케이프하면 리터럴 `${...}`로 전달된다

### Setup / Teardown

```markdown
<!-- setup -->
<!-- teardown -->
```

이 지시자는 현재 heading subtree 전체에 적용된다.
실제 훅으로 바꾸는 책임은 runtime adapter에 있다.


## 표 기반 명세

FIT 스타일의 핵심은 유지하되, 코어는 fixture adapter 구조로 일반화한다.

예시:

````markdown
<!-- fixture:write-permission(user=alan) -->
| path                       | write | reason                |
|----------------------------|-------|-----------------------|
| /private/test.txt          | yes   | 개인 작업 공간        |
| /MEMORY.md                 | yes   | 실행 간 기억 지속     |
| /channels/general/chat.log | no    | 채널은 post로만 기록  |
````

규칙:

- table은 바로 위의 `fixture` directive와 결합될 때만 executable이다
- 첫 행은 반드시 header다
- 각 fixture adapter는 필요한 컬럼을 명시적으로 검증해야 한다
- unknown fixture는 compile-time error다
- 각 row는 독립된 test case이자 독립된 report row가 된다

fixture adapter contract는 다음 요구를 만족해야 한다.

- 입력 table을 row 단위 실행 계획으로 확장할 수 있어야 한다
- 실패 시 어느 row가 왜 실패했는지 `SpecId`와 함께 reportable해야 한다
- 가능한 경우 expected/actual diff를 구조화해서 제공해야 한다


## Literate Alloy

Alloy는 문서 안에 자연어와 weaving 되는 것이 중요하다.
v1에서는 다음 규칙으로 고정한다.

### 임베딩 규칙

`alloy:model(name)`는 logical model `name`에 속한 fragment다.
같은 `name`을 가진 fragment는 문서 순서대로 결합된다.

예시:

````markdown
텍스트로 개념을 설명한다.

```alloy:model(access)
module access

sig Node {}
sig Path {}
```

private 규칙의 이유를 설명한다.

```alloy:model(access)
sig PrivatePath in Path {}

assert privateIsolation {
  all p: PrivatePath | ...
}

check privateIsolation for 5
```
````

### 결합 규칙

- 같은 model name의 fragment는 하나의 virtual `.als` 파일로 합쳐진다
- 첫 fragment만 `module` 선언을 포함할 수 있다
- 이후 fragment에 `module`이 다시 나오면 compile-time error다
- 생성된 model에는 source mapping comment를 삽입한다
  - 예: `-- specdown-source: docs/foo.spec.md#Access/Isolation`

### 모델 참조

문서 독자가 어떤 assertion이 검증되었는지 쉽게 볼 수 있도록 explicit directive를 둔다.

```markdown
<!-- alloy:ref(access#privateIsolation, scope=5) -->
```

이 directive는 다음 역할을 한다.

- 현재 문단/섹션과 특정 model check 결과를 연결
- HTML report에 badge 또는 status row로 노출
- 실패 시 대응 counterexample artifact로 링크

자연어 blockquote는 자유롭게 써도 되지만, machine-readable contract는 위 directive를 기준으로 한다.


## 실행 결과 HTML 뷰

HTML 리포트는 v1의 핵심 deliverable이다.
목표는 "테스트 로그"가 아니라 "실행된 명세서"를 보여 주는 것이다.

### 기본 요구사항

- heading 구조를 그대로 유지한 문서 레이아웃
- prose는 그대로 표시하고, 실행 결과만 상태로 주석화
- section, code block, table row, alloy reference 단위 상태 표시
- pass는 녹색 배경 또는 badge
- fail은 붉은 배경 또는 badge
- fail 항목에는 기대값/실제값/오류 메시지/stack trace/stdout/stderr를 인라인 또는 펼침 패널로 표시
- summary pane 제공
  - 총 실행 수
  - pass/fail 수
  - 실패 목록
  - model check 결과

### 아티팩트 요구사항

최소 산출물:

- `.artifacts/specdown/report.html`
- `.artifacts/specdown/report.json`
- `.artifacts/specdown/models/*.als`
- `.artifacts/specdown/counterexamples/*` (실패 시)

### UX 원칙

- JavaScript가 없어도 본문과 주요 실패 정보는 읽혀야 한다
- anchor link로 원문 heading에 바로 이동 가능해야 한다
- 실패 row와 실패 block은 fold/unfold가 가능해야 한다
- 동일 문서의 prose와 결과를 분리하지 않는다


## CLI

독립 팀이 구현할 CLI 표면은 다음 정도면 충분하다.

```bash
specdown run                          # 기본 실행
specdown run -filter "보드" -jobs 4   # 필터링 + 병렬 실행
specdown run -dry-run                 # 파싱·검증만 수행
specdown version                      # 버전 출력
specdown alloy dump                   # Alloy 모델 .als 파일만 생성
```

의미:

- `run`: Markdown parse, adapter 실행, embedded Alloy 검사, model bundle 생성, report artifact 생성을 한 번에 수행한다
- `version`: 빌드 시 주입된 버전 문자열을 출력한다
- `alloy dump`: adapter 실행 없이 Alloy 모델 파일만 생성한다

v1에서는 `specdown run`이 compile + execute + report를 한 번에 수행하는 기본 경로다.

실패 시 각 실패 항목의 heading path, 블록/fixture 이름, 오류 메시지를 stderr에 출력한 뒤 요약을 표시한다.


## 구현 단계

독립 팀에 전달할 구현 순서는 다음으로 고정한다.

### Phase 1: Core 문법과 serializable event schema 고정

목표:

- parser, AST, `SpecId`, execution plan, event schema를 고정
- adapter에 넘길 수 있는 JSON-serializable node shape를 고정

산출물:

- `specdown-core`
- execution plan / event schema 문서
- compile-time error 규칙 문서

### Phase 2: Adapter protocol과 host 고정

목표:

- adapter 경계를 early phase에 고정해 프로젝트별 확장이 코어 변경 없이 가능하게 함
- 각 프로젝트가 원하는 언어로 최소 노력 adapter를 만들 수 있게 함

산출물:

- `specdown-adapter-protocol`
- `specdown-cli` adapter launcher
- `specdown.json` loader
- stdin/stdout NDJSON protocol 문서
- 서로 다른 두 언어로 작성된 minimal reference adapter 2개

### Phase 3: HTML reporter

목표:

- event stream만으로 문서 중심 HTML 리포트 생성

산출물:

- `specdown-reporter-html`
- `report.json` schema
- anchorable static HTML artifact

### Phase 4: Optional built-in generic adapters

목표:

- 범용성이 높은 adapter만 builtin package로 제공
- architecture는 builtin adapter 없이도 성립하게 유지

산출물:

- `specdown-adapter-shell` 같은 optional adapter 1~2개
- optional helper SDK 또는 adapter template

### Phase 5: Table fixtures

목표:

- FIT 스타일 표 명세를 fixture adapter로 일반화

산출물:

- fixture adapter protocol extension
- sample fixtures 2~3개
- row-level reporting

### Phase 6: Alloy support

목표:

- literate Alloy fragment 추출, bundle 생성, model check 연결

산출물:

- `specdown-alloy`
- embedded model source mapping
- counterexample artifact wiring

### Phase 7: Reference product adapter

목표:

- 특정 제품의 DSL을 adapter로 분리해 통합 예시 제공

산출물:

- reference adapter package 1개
- reference specs 1~3개 migration


## 수용 기준

독립 팀 handoff 기준의 완료 조건은 다음과 같다.

1. `specdown-core`가 특정 테스트 프레임워크나 제품 코드에 의존하지 않는다.
2. 하나의 `.spec.md` 문서에 prose, executable block, fixture table, Alloy fragment를 함께 쓸 수 있다.
3. 같은 model name을 가진 `alloy:model(...)` fragment가 문서 순서대로 결합된다.
4. HTML report에서 section, block, row, alloy check 상태가 각각 보인다.
5. failure detail이 문서 컨텍스트를 잃지 않고 표시된다.
6. 프로젝트는 data-only config에 adapter command 하나만 등록해도 문서를 실행할 수 있다.
7. adapter는 stdin/stdout protocol만 따르면 어떤 언어로도 구현할 수 있다.
8. 제품 특화 DSL과 helper는 core가 아니라 adapter에만 존재한다.


## 예시

### 문서 예시

````markdown
## Write permissions

노드는 최소 권한 원칙을 따른다.

<!-- alloy:ref(access#writeMinimality, scope=5) -->

<!-- fixture:write-permission(user=alan) -->
| path                       | write | reason                |
|----------------------------|-------|-----------------------|
| /private/test.txt          | yes   | 개인 작업 공간        |
| /MEMORY.md                 | yes   | 실행 간 기억 지속     |
| /channels/general/chat.log | no    | 채널은 post로만 기록  |
````

### Alloy weaving 예시

````markdown
## Private Isolation

private 경로는 소유자 본인만 읽을 수 있어야 한다.

```alloy:model(access)
module access

sig Node {}
sig Path { owner: one Node }
sig PrivatePath in Path {}
```

위 개념에서 "읽기 가능" 관계를 도입한다.

```alloy:model(access)
pred canRead[n: Node, p: Path] {
  p not in PrivatePath or p.owner = n
}

assert privateIsolation {
  all n1, n2: Node |
    n1 != n2 implies
      all p: PrivatePath | p.owner = n1 implies not canRead[n2, p]
}

check privateIsolation for 5
```

<!-- alloy:ref(access#privateIsolation, scope=5) -->
````


## 평가

| 기준 | 기존 방식 | `specdown` |
|------|----------|------------|
| 문서 가독성 | 설계와 테스트가 분리됨 | 하나의 literate spec 문서 |
| 형식 검증 | 별도 모델 없거나 수동 | Alloy로 연결 |
| 테스트 추가 비용 | 코드 작성 필요 | 표 행 추가 또는 블록 추가 |
| 결과 가시성 | 테스트 로그 중심 | HTML 문서 중심 |
| 제품 독립성 | 제품마다 새로 구현 | core + adapter 구조 |
| 재사용성 | 낮음 | 런타임/리포터/DSL 교체 가능 |


## 결론

`specdown`은 "Markdown 기반 literate specification + embedded Alloy + FIT 스타일 표 명세 + HTML 실행 리포트"를 reusable core로 제공하는 시스템이다.

독립 팀은 이 문서를 기준으로 core와 adapter 경계를 분명히 유지한 채 바로 구현을 시작할 수 있어야 한다.
