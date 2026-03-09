# Pocket Card

A card is the unit for tracking work within a board.

First, create a board.

```run:board -> $boardName
create-board
```

When a card is created, the system must return a new card identifier.

```run:board -> $cardId
create-card "${boardName}" "write spec"
```

## Card Lookup

A card just created must be immediately queryable within the board it was created in.

> fixture:card-exists
| board | card | exists |
| --- | --- | --- |
| ${boardName} | ${cardId} | yes |

A newly created card must always be placed in the `todo` column.

> fixture:card-column
| board | card | column |
| --- | --- | --- |
| ${boardName} | ${cardId} | todo |

## Card Movement

A card must be movable to another column to reflect its current work status.

```run:board
move-card "${boardName}" "${cardId}" doing
```

### Move to doing

A card moved to `doing` must be queryable under the `doing` column in the same board.

> fixture:card-column
| board | card | column |
| --- | --- | --- |
| ${boardName} | ${cardId} | doing |

### Move to done

A completed card must be movable to `done`.

```run:board
move-card "${boardName}" "${cardId}" done
```

> fixture:card-column
| board | card | column |
| --- | --- | --- |
| ${boardName} | ${cardId} | done |

### Moving to a nonexistent column should fail

Moving to an undefined column name must return an error.

```run:board
moving "${cardId}" to "invalid" should fail
```

### Moving to the same column is not an error

Moving to the column the card is already in must be handled without error.

```run:board
moving "${cardId}" to current column should succeed
```

## Card Title

### Title must not be empty

Creating a card with an empty title must be rejected.

```run:board
card with empty title should be rejected
```

### Title length must be at most 256 characters

Titles of 257 characters or more must be rejected.

```run:board
card title length must be at most 256
```

### Title can be modified

It must be possible to change the title of an existing card.

```run:board
rename-card "${boardName}" "${cardId}" "spec complete"
```

```run:board
card "${cardId}" title should be "spec complete"
```

## Card Deletion

### An existing card can be deleted

Once a card is deleted, it must no longer be queryable.

```run:board
delete-card "${boardName}" "${cardId}"
```

### A deleted card is not queryable

> fixture:card-exists
| board | card | exists |
| --- | --- | --- |
| ${boardName} | ${cardId} | no |

### Deleting a nonexistent card should fail

Attempting to delete a card that was never created must return an error.

```run:board
deleting nonexistent card should fail
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

