# Pocket Card

카드는 보드 안에서 작업을 추적하는 단위다.

먼저 보드를 만든다.

```run:board -> $boardName
create-board
```

카드를 만들면 시스템은 새 카드 식별자를 반환해야 한다.

```run:board -> $cardId
create-card "${boardName}" "명세 쓰기"
```

## 카드 조회

방금 생성한 카드는 생성한 보드 안에서 바로 조회 가능해야 한다.

<!-- fixture:card-exists -->
| board | card | exists |
| --- | --- | --- |
| ${boardName} | ${cardId} | 예 |

새로 만든 카드는 항상 `todo` 컬럼에 놓여야 한다.

<!-- fixture:card-column -->
| board | card | column |
| --- | --- | --- |
| ${boardName} | ${cardId} | todo |

## 카드 이동

카드는 현재 작업 상태를 반영하도록 다른 컬럼으로 이동할 수 있어야 한다.

```run:board
move-card "${boardName}" "${cardId}" doing
```

### doing으로 이동

`doing`으로 이동한 카드는 같은 보드에서 `doing` 컬럼으로 조회되어야 한다.

<!-- fixture:card-column -->
| board | card | column |
| --- | --- | --- |
| ${boardName} | ${cardId} | doing |

### done으로 이동

작업이 끝난 카드는 `done`으로 이동할 수 있어야 한다.

```run:board
move-card "${boardName}" "${cardId}" done
```

<!-- fixture:card-column -->
| board | card | column |
| --- | --- | --- |
| ${boardName} | ${cardId} | done |

### 존재하지 않는 컬럼으로 이동하면 오류

정의되지 않은 컬럼 이름으로 이동하면 오류를 반환해야 한다.

```verify:board
moving "${cardId}" to "invalid" should fail
```

### 같은 컬럼으로 이동해도 오류가 아니다

이미 있는 컬럼으로 다시 이동해도 정상 처리되어야 한다.

```verify:board
moving "${cardId}" to current column should succeed
```

## 카드 제목

### 제목은 빈 문자열일 수 없다

카드 제목이 비어 있으면 생성이 거부되어야 한다.

```verify:board
card with empty title should be rejected
```

### 제목 길이는 256자 이하여야 한다

257자 이상의 제목은 거부되어야 한다.

```verify:board
card title length must be at most 256
```

### 제목은 수정할 수 있다

기존 카드의 제목을 변경할 수 있어야 한다.

```run:board
rename-card "${boardName}" "${cardId}" "명세 완성"
```

```verify:board
card "${cardId}" title should be "명세 완성"
```

## 카드 삭제

### 존재하는 카드를 삭제할 수 있다

카드를 삭제하면 더 이상 조회되지 않아야 한다.

```run:board
delete-card "${boardName}" "${cardId}"
```

### 삭제한 카드는 조회되지 않는다

<!-- fixture:card-exists -->
| board | card | exists |
| --- | --- | --- |
| ${boardName} | ${cardId} | 아니오 |

### 존재하지 않는 카드를 삭제하면 오류

한 번도 만들지 않은 카드를 삭제하려 하면 오류를 반환해야 한다.

```verify:board
deleting nonexistent card should fail
```

## 형식 규칙

`Pocket Board`의 상태 모델은 카드가 항상 하나의 컬럼에만 속한다고 본다.

```alloy:model(board)
module board

abstract sig Column {}
one sig Todo, Doing, Done extends Column {}

sig Board {}

sig Card {
  board: one Board,
  column: one Column
}
```

이 모델에서 각 카드는 정확히 하나의 컬럼을 가져야 한다.

```alloy:model(board)
assert cardHasExactlyOneColumn {
  all c: Card | one c.column
}

check cardHasExactlyOneColumn for 5
```

<!-- alloy:ref(board#cardHasExactlyOneColumn, scope=5) -->

### 카드는 정확히 하나의 보드에 속해야 한다

카드가 여러 보드에 동시에 속하는 것은 불가능해야 한다.

```alloy:model(board)
assert cardBelongsToOneBoard {
  all c: Card | one c.board
}

check cardBelongsToOneBoard for 5
```

<!-- alloy:ref(board#cardBelongsToOneBoard, scope=5) -->

### 의도적 반례

의도적으로 틀린 단언: 카드가 둘 이상의 컬럼을 가질 수 있다고 주장한다.

```alloy:model(board)
assert cardCanHaveMultipleColumns {
  some c: Card | #c.column > 1
}

check cardCanHaveMultipleColumns for 5
```

<!-- alloy:ref(board#cardCanHaveMultipleColumns, scope=5) -->
