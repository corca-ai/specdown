# Pocket Card

A card is the unit for tracking work within a board.

First, create a board.

```run:shell -> $boardName
rm -f .board-state.json
python3 tools/board.py create-board
```

When a card is created, the system must return a new card identifier.

```run:shell -> $cardId
python3 tools/board.py create-card "${boardName}" "write spec"
```

## Card Lookup

A card just created must be immediately queryable within the board it was created in.

```run:shell
$ python3 tools/board.py card-exists "${boardName}" "${cardId}"
yes
```

A newly created card must always be placed in the `todo` column.

```run:shell
$ python3 tools/board.py card-column "${boardName}" "${cardId}"
todo
```

## Card Movement

A card must be movable to another column to reflect its current work status.

```run:shell
python3 tools/board.py move-card "${boardName}" "${cardId}" doing
```

### Move to doing

A card moved to `doing` must be queryable under the `doing` column in the same board.

```run:shell
$ python3 tools/board.py card-column "${boardName}" "${cardId}"
doing
```

### Move to done

A completed card must be movable to `done`.

```run:shell
python3 tools/board.py move-card "${boardName}" "${cardId}" done
```

```run:shell
$ python3 tools/board.py card-column "${boardName}" "${cardId}"
done
```

### Moving to a nonexistent column should fail

Moving to an undefined column name must return an error.

```run:shell
! python3 tools/board.py move-card "${boardName}" "${cardId}" invalid 2>/dev/null
```

### Moving to the same column is not an error

Moving to the column the card is already in must be handled without error.

```run:shell
python3 tools/board.py move-card "${boardName}" "${cardId}" done
```

## Card Title

### Title must not be empty

Creating a card with an empty title must be rejected.

```run:shell
! python3 tools/board.py create-card "${boardName}" "" 2>/dev/null
```

### Title length must be at most 256 characters

Titles of 257 characters or more must be rejected.

```run:shell
! python3 tools/board.py create-card "${boardName}" "$(python3 -c "print('a'*257)")" 2>/dev/null
```

### Title can be modified

It must be possible to change the title of an existing card.

```run:shell
python3 tools/board.py rename-card "${boardName}" "${cardId}" "spec complete"
```

```run:shell
$ python3 tools/board.py card-title "${boardName}" "${cardId}"
spec complete
```

## Card Deletion

### An existing card can be deleted

Once a card is deleted, it must no longer be queryable.

```run:shell
python3 tools/board.py delete-card "${boardName}" "${cardId}"
```

### A deleted card is not queryable

```run:shell
$ python3 tools/board.py card-exists "${boardName}" "${cardId}"
no
```

### Deleting a nonexistent card should fail

Attempting to delete a card that was never created must return an error.

```run:shell
! python3 tools/board.py delete-card "${boardName}" nonexistent 2>/dev/null
```

## Formal Rules

The state model of `Pocket Board` assumes that a card always belongs to exactly one column.

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

In this model, each card must have exactly one column.

```alloy:model(board)
assert cardHasExactlyOneColumn {
  all c: Card | one c.column
}

check cardHasExactlyOneColumn for 5
```

### A card must belong to exactly one board

It must be impossible for a card to belong to multiple boards simultaneously.

```alloy:model(board)
assert cardBelongsToOneBoard {
  all c: Card | one c.board
}

check cardBelongsToOneBoard for 5
```
