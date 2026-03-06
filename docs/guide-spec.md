# Writing Specs

Spec 파일은 `*.spec.md` Markdown 문서다.
산문은 그대로 보존되고, 특정 블록과 표만 실행된다.

## Heading 구조

Heading hierarchy가 테스트 suite 계층이 된다.

```markdown
# 제품 이름          ← 최상위 suite
## 기능 A            ← 하위 suite
### 시나리오 1       ← 개별 시나리오
```

## Executable Block

fenced code block의 info string으로 실행 블록을 표시한다.

````markdown
```run:board -> $boardName
create-board
```
````

| 접두사 | 의미 |
|--------|------|
| `run:<target>` | 부수 효과가 있는 실행 블록 |
| `verify:<target>` | 단언 블록 |

`<target>`은 adapter가 정의한다. `-> $변수명`으로 결과를 캡처할 수 있다.

## 변수

블록에서 캡처한 값을 이후 블록과 표에서 `${변수명}`으로 참조한다.

````markdown
```run:api -> $userId
POST /users {"name": "alice"}
```

```verify:api
GET /users/${userId}
```
````

규칙:
- 스코프는 같은 heading subtree 안으로 제한
- 상위 섹션 변수는 하위에서 읽을 수 있음
- 형제 섹션 간 공유 불가
- 미해결 변수는 오류

## Fixture Table

HTML 주석으로 fixture를 지정하고, 바로 아래 Markdown 표를 연결한다.

```markdown
<!-- fixture:board-exists -->
| board        | exists |
|--------------|--------|
| ${boardName} | yes    |
```

- 첫 행은 header
- 각 행이 독립된 test case
- fixture 이름은 adapter가 정의

## Alloy Model

Alloy fragment를 문서에 직접 포함할 수 있다.

````markdown
```alloy:model(board)
module board

sig Card { column: one Column }
```
````

같은 이름의 fragment는 문서 순서대로 결합된다.
`module` 선언은 첫 fragment에만 쓴다.

assertion 검증 결과를 문서에 연결하려면:

```markdown
<!-- alloy:ref(board#cardHasExactlyOneColumn, scope=5) -->
```
