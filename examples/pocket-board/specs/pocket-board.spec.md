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

```run:board
board "${boardName}" should exist
```

### A board that was not created must not exist

A board name that was never created must not exist.

```run:board
board "${boardName}-archive" should not exist
```

### Board Existence Rules

Board existence can be independently verified for each row in a table.

> check:board-exists
| board | exists |
| --- | --- |
| ${boardName} | yes |
| ${boardName}-archive | no |

## Board Name Rules

Board names are subject to validation rules.

```run:board
board "invalid name" should be rejected
```

```run:board
board name length must be at most 64
```

```run:board
duplicate board should be rejected
```

## Board Deletion

Deleting a created board must make it no longer queryable.

```run:board
delete-board "${boardName}"
```

The deleted board must no longer exist.

```run:board
board "${boardName}" should not exist
```

Attempting to delete a board that was never created must return an error.

```run:board
deleting nonexistent board should fail
```

## Board List

After creating a new board, we can test list behavior.

```run:board -> $boardName2
create-board
```

The board list must contain at least one entry.

```run:board
board list should contain at least one entry
```

When multiple boards exist, the list must be sorted alphabetically by name.

```run:board
board list should be sorted alphabetically
```
