# Pocket Board

`Pocket Board` is a small kanban board for organizing personal tasks.

## Product Overview

A board has exactly three columns.
The column names are `todo`, `doing`, and `done`.

Each board is identified by a unique name.
A card has an identifier, a title, and a current column.

New cards start in `todo`.
Cards in progress are placed in `doing`.
Completed cards are moved to `done`.

## Board Creation

When a board is created, the system must return a new board name.
The returned name must be referenceable in subsequent commands and verifications.

```run:board -> $boardName
create-board
```

### A created board must exist immediately

A board just created must be immediately queryable.

```verify:board
board "${boardName}" should exist
```

### A board that was not created must not exist

A board name that was never created must not exist.

```verify:board
board "${boardName}-archive" should not exist
```

### An archive copy of a created board must also be queryable

When a board is created, an archive copy must also be queryable.

```verify:board
board "${boardName}-archive" should exist
```

### Board Existence Rules

Board existence can be independently verified for each row in a table.

<!-- fixture:board-exists -->
| board | exists |
| --- | --- |
| ${boardName} | yes |
| ${boardName}-archive | no |

## Board Name Rules

### Name must not contain spaces

Board names with spaces must be rejected.

```verify:board
board "invalid name" should be rejected
```

### Name length must be at most 64 characters

Names of 65 characters or more must be rejected.

```verify:board
board name length must be at most 64
```

### Duplicate names must be rejected

Creating a board with an already existing name must return an error.

```verify:board
duplicate board should be rejected
```

## Board Deletion

### An existing board can be deleted

Deleting a created board must make it no longer queryable.

```run:board
delete-board "temp-board"
```

### A deleted board is not queryable

Querying a deleted board must respond that it does not exist.

```verify:board
board "temp-board" should not exist
```

### Deleting a nonexistent board should fail

Attempting to delete a board that was never created must return an error.

```verify:board
deleting nonexistent board should fail
```

## Board List

### A created board is included in the list

After creating a board, querying the full list must include that board.

```verify:board
board list should contain at least one entry
```

### The list is sorted by name

When multiple boards exist, the list must be sorted alphabetically by name.

```verify:board
board list should be sorted alphabetically
```
