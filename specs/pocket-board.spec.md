# Pocket Board

`Pocket Board`는 `specdown`을 점진적으로 개발할 때 함께 키워 가는 장난감 프로젝트다.

Phase 0에서는 평범한 자연어 문서만 있었다.
Phase 1에서는 그 산문을 유지한 채 첫 executable block을 추가했다.
이제 문서는 executable block, 변수 바인딩, fixture table까지 포함한다.

## 제품 개요

`Pocket Board`는 개인 작업을 위한 아주 작은 칸반 보드다.
컬럼은 정확히 세 개뿐이다. `todo`, `doing`, `done`.
프로젝트 크기를 의도적으로 작게 유지해서 명세 시스템 자체를 무겁게 만들지 않으려 한다.

## 왜 이 프로젝트가 적합한가

이 프로젝트는 한 번에 이해할 수 있을 만큼 작다.
그러면서도 실행 블록, 변수 흐름, 표 기반 검증, 이후의 형식 모델까지 자연스럽게 얹을 여지는 충분하다.

이 저장소에서 다음 기능을 검증하는 예제로 쓴다.

- executable block 실행
- adapter state 유지
- 변수 캡처와 치환
- fixture table의 row-level reporting
- 실패 요약이 포함된 HTML 리포트

## 핵심 개념

보드는 카드를 담는다.
각 카드는 식별자, 제목, 현재 컬럼을 가진다.

카드는 `todo`에서 시작한다.
진행 중인 일은 `doing`에 놓인다.
끝난 일은 `done`으로 이동한다.

## 동작 의도

이 시스템은 예측 가능해야 한다.
사용자는 카드가 어디에 있고 지금 어떤 상태인지 항상 알 수 있어야 한다.

허용 전이는 작고 명확해야 한다.
아직 카드 전이 규칙 전체를 형식화하지는 않았지만, 그 기반이 되는 실행 가능한 예시를 이 문서에서 점진적으로 늘린다.

## 현재 상태

이 저장소에서는 Phase 0, Phase 1, Phase 2, Phase 3이 완료되었다.
그리고 그 위에 첫 변수 바인딩 흐름과 첫 fixture table 흐름이 추가되었다.

현재 구현은 다음을 수행한다.

- `.spec.md` 문서를 찾는다
- 문서를 heading, prose, fenced code block, fixture table로 파싱한다
- `run:board`, `verify:board`, `fixture:board-exists`를 외부 adapter command로 실행한다
- 문서 순서대로 adapter state를 유지한다
- `run:* -> $name` 캡처와 `${name}` 치환을 지원한다
- HTML 리포트에 block과 table row 상태를 인라인으로 표시한다
- 실패한 case를 요약 섹션에 모아서 보여 준다
- 하나라도 실패하면 non-zero로 종료하되 리포트는 남긴다

## 다음 목표

이 문서는 교체하지 않고 계속 확장한다.
다음 단계에서는 더 많은 fixture와, eventually Alloy fragment도 이 문서 위에 얹을 수 있어야 한다.

## 변수 흐름

첫 실행 동작은 값을 하나 캡처하는 것이다.
adapter는 새 보드 이름을 만들고, 그 보드를 생성하고, 결과를 `$boardName`에 바인딩해야 한다.

```run:board -> $boardName
create-board
```

이 블록이 통과하면 `specdown`은 passing case result를 기록하고, 캡처한 값을 이후 블록과 표에서 재사용할 수 있어야 한다.

### 생성한 보드 확인

앞선 블록이 만든 보드는 뒤이은 verification block에서 참조 가능해야 한다.

```verify:board
board "${boardName}" should exist
```

이 블록은 통과해야 한다.
이로써 verification이 이전 `run:board` 블록이 만든 상태를 읽을 수 있음을 확인한다.

### 표 기반 확인

Phase 5의 가장 작은 형태는 fixture table이다.
같은 보드 상태를 표의 각 행에서 독립적으로 검증해야 한다.

<!-- fixture:board-exists -->
| board | exists |
| --- | --- |
| ${boardName} | 예 |
| ${boardName}-archive | 예 |

첫 번째 행은 통과해야 한다.
두 번째 행은 의도적으로 실패해야 한다.

`specdown run`은 따라서 non-zero로 종료해야 하지만, HTML 리포트는 정상적으로 생성되어야 하고, 실패한 행으로 바로 이동할 수 있어야 한다.
