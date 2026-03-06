# Pocket Board

`Pocket Board`는 개인 작업을 정리하기 위한 작은 칸반 보드다.

## 제품 개요

보드는 정확히 세 개의 컬럼을 가진다.
컬럼 이름은 `todo`, `doing`, `done`이다.

각 보드는 고유한 이름으로 식별된다.
카드는 식별자, 제목, 현재 컬럼을 가진다.

새 카드는 `todo`에서 시작한다.
작업 중인 카드는 `doing`에 놓인다.
완료된 카드는 `done`으로 이동한다.

## 보드 생성

보드를 만들면 시스템은 새 보드 이름을 반환해야 한다.
반환된 이름은 이후 명령과 검증에서 다시 참조할 수 있어야 한다.

```run:board -> $boardName
create-board
```

### 생성한 보드는 즉시 존재해야 한다

방금 생성한 보드는 바로 조회 가능해야 한다.

```verify:board
board "${boardName}" should exist
```

### 생성하지 않은 보드는 존재하지 않아야 한다

한 번도 만들지 않은 보드 이름은 존재하지 않아야 한다.

```verify:board
board "${boardName}-archive" should not exist
```

### 보드 존재 규칙

보드의 존재 여부는 표의 각 행에서 독립적으로 검증할 수 있어야 한다.

<!-- fixture:board-exists -->
| board | exists |
| --- | --- |
| ${boardName} | 예 |
| ${boardName}-archive | 아니오 |

### 카드 생성

카드를 만들면 시스템은 새 카드 식별자를 반환해야 한다.
반환된 식별자는 이후 명령과 검증에서 다시 참조할 수 있어야 한다.

```run:board -> $cardId
create-card "${boardName}" "명세 쓰기"
```

#### 생성한 카드는 즉시 존재해야 한다

방금 생성한 카드는 생성한 보드 안에서 바로 조회 가능해야 한다.

<!-- fixture:card-exists -->
| board | card | exists |
| --- | --- | --- |
| ${boardName} | ${cardId} | 예 |

#### 새 카드는 todo 컬럼에서 시작해야 한다

새로 만든 카드는 항상 `todo` 컬럼에 놓여야 한다.

<!-- fixture:card-column -->
| board | card | column |
| --- | --- | --- |
| ${boardName} | ${cardId} | todo |

#### 카드 이동

카드는 현재 작업 상태를 반영하도록 다른 컬럼으로 이동할 수 있어야 한다.

```run:board
move-card "${boardName}" "${cardId}" doing
```

##### 이동한 카드는 새 컬럼에 있어야 한다

`doing`으로 이동한 카드는 같은 보드에서 `doing` 컬럼으로 조회되어야 한다.

<!-- fixture:card-column -->
| board | card | column |
| --- | --- | --- |
| ${boardName} | ${cardId} | doing |
