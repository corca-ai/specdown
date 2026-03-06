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

`doing`으로 이동한 카드는 같은 보드에서 `doing` 컬럼으로 조회되어야 한다.

<!-- fixture:card-column -->
| board | card | column |
| --- | --- | --- |
| ${boardName} | ${cardId} | doing |

작업 중인 카드는 동시에 `done` 후보 컬럼에서도 조회될 수 있어야 한다.

<!-- fixture:card-column -->
| board | card | column |
| --- | --- | --- |
| ${boardName} | ${cardId} | done |

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

의도적으로 틀린 단언: 카드가 둘 이상의 컬럼을 가질 수 있다고 주장한다.

```alloy:model(board)
assert cardCanHaveMultipleColumns {
  some c: Card | #c.column > 1
}

check cardCanHaveMultipleColumns for 5
```

<!-- alloy:ref(board#cardCanHaveMultipleColumns, scope=5) -->
