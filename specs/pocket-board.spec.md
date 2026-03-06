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

### 생성한 보드의 보관 사본도 조회되어야 한다

보드를 만들면 보관 사본도 함께 조회 가능해야 한다.

```verify:board
board "${boardName}-archive" should exist
```

### 보드 존재 규칙

보드의 존재 여부는 표의 각 행에서 독립적으로 검증할 수 있어야 한다.

<!-- fixture:board-exists -->
| board | exists |
| --- | --- |
| ${boardName} | 예 |
| ${boardName}-archive | 아니오 |

## 보드 이름 규칙

### 이름에 공백을 포함할 수 없다

보드 이름에 공백이 있으면 생성이 거부되어야 한다.

```verify:board
board "invalid name" should be rejected
```

### 이름 길이는 64자 이하여야 한다

65자 이상의 이름은 거부되어야 한다.

```verify:board
board name length must be at most 64
```

### 중복 이름은 거부되어야 한다

이미 존재하는 보드 이름으로 다시 생성하면 오류를 반환해야 한다.

```verify:board
duplicate board should be rejected
```

## 보드 삭제

### 존재하는 보드를 삭제할 수 있다

생성한 보드를 삭제하면 더 이상 조회되지 않아야 한다.

```run:board
delete-board "temp-board"
```

### 삭제한 보드는 조회되지 않는다

삭제된 보드를 조회하면 존재하지 않는다고 응답해야 한다.

```verify:board
board "temp-board" should not exist
```

### 존재하지 않는 보드를 삭제하면 오류

한 번도 만들지 않은 보드를 삭제하려 하면 오류를 반환해야 한다.

```verify:board
deleting nonexistent board should fail
```

## 보드 목록

### 생성한 보드가 목록에 포함된다

보드를 생성한 뒤 전체 목록을 조회하면 해당 보드가 포함되어야 한다.

```verify:board
board list should contain at least one entry
```

### 목록은 이름순으로 정렬된다

여러 보드가 있을 때 목록은 이름의 사전순으로 정렬되어야 한다.

```verify:board
board list should be sorted alphabetically
```
