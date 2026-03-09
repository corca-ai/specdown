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

```run:shell -> $boardName
rm -f .board-state.json
python3 tools/board.py create-board
```

### A created board must exist immediately

A board just created must be immediately queryable.

```run:shell
$ python3 tools/board.py board-exists "${boardName}"
yes
```

### A board that was not created must not exist

A board name that was never created must not exist.

```run:shell
$ python3 tools/board.py board-exists "${boardName}-archive"
no
```

## Board Name Rules

Board names are subject to validation rules.

Board names must not contain spaces.

```run:shell
! python3 tools/board.py create-board "invalid name" 2>/dev/null
```

Board names must be at most 64 characters.

```run:shell
! python3 tools/board.py create-board "$(python3 -c "print('a'*65)")" 2>/dev/null
```

Duplicate board names are rejected.

```run:shell
! python3 tools/board.py create-board "${boardName}" 2>/dev/null
```

## Board Deletion

Deleting a created board must make it no longer queryable.

```run:shell
python3 tools/board.py delete-board "${boardName}"
```

The deleted board must no longer exist.

```run:shell
$ python3 tools/board.py board-exists "${boardName}"
no
```

Attempting to delete a board that was never created must return an error.

```run:shell
! python3 tools/board.py delete-board nonexistent 2>/dev/null
```

## Board List

After creating a new board, we can test list behavior.

```run:shell -> $boardName2
python3 tools/board.py create-board
```

The board list must contain at least one entry.

```run:shell
test "$(python3 tools/board.py list-boards | wc -l)" -gt 0
```

When multiple boards exist, the list must be sorted alphabetically.

```run:shell
boards=$(python3 tools/board.py list-boards)
sorted=$(echo "$boards" | sort)
test "$boards" = "$sorted"
```
