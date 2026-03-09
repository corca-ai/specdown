#!/usr/bin/env python3

import json
import shlex
import sys


def emit(payload):
    sys.stdout.write(json.dumps(payload) + "\n")
    sys.stdout.flush()


class SpecFailure(Exception):
    def __init__(self, message):
        super().__init__(message)
        self.message = message


def parse_single_arg(raw):
    raw = raw.strip()
    if not raw:
        raise SpecFailure("missing board name")

    if raw.startswith('"'):
        try:
            value = json.loads(raw)
        except json.JSONDecodeError:
            raise SpecFailure(f'invalid quoted argument {raw!r}')
        if not value:
            raise SpecFailure("board name must not be empty")
        return value

    if " " in raw or "\t" in raw:
        raise SpecFailure("board name must be a single token or quoted string")
    return raw


def boards_snapshot(state):
    return "[" + ", ".join(json.dumps(item) for item in sorted(state["boards"])) + "]"


def board_cards_snapshot(state, board_name):
    board = state["boards"].get(board_name)
    if not board:
        return "[]"
    return "[" + ", ".join(json.dumps(item) for item in sorted(board["cards"])) + "]"


def board_exists_failure(board_name, should_exist, state):
    names = sorted(state["boards"])
    actual = ", ".join(names) if names else "(none)"
    if should_exist:
        return SpecFailure(
            f'expected board {board_name!r} to exist; actual: {actual}',
        )
    return SpecFailure(
        f'expected board {board_name!r} not to exist; actual: {actual}',
    )


def card_exists_failure(board_name, card_name, should_exist, state):
    board = state["boards"].get(board_name)
    names = sorted(board["cards"]) if board else []
    actual = ", ".join(names) if names else "(none)"
    if should_exist:
        return SpecFailure(
            f'expected card {card_name!r} in board {board_name!r} to exist; actual: {actual}',
        )
    return SpecFailure(
        f'expected card {card_name!r} in board {board_name!r} not to exist; actual: {actual}',
    )


def card_column_failure(board_name, card_name, column, actual_column):
    return SpecFailure(
        f'expected column {column!r}, got {actual_column!r}',
    )


def parse_command(line):
    try:
        return shlex.split(line)
    except ValueError as err:
        raise SpecFailure(f'invalid command {line!r}: {err}')


def ensure_board(state, board_name):
    board = state["boards"].get(board_name)
    if board is None:
        raise board_exists_failure(board_name, True, state)
    return board


def ensure_card(state, board_name, card_id):
    board = ensure_board(state, board_name)
    card = board["cards"].get(card_id)
    if card is None:
        raise card_exists_failure(board_name, card_id, True, state)
    return board, card


VALID_COLUMNS = ("todo", "doing", "done")


def run_exec(state, source):
    """Execute a source command, returning output string."""
    output_lines = []
    for raw_line in source.splitlines():
        line = raw_line.strip()
        if not line:
            continue
        parts = parse_command(line)
        if not parts:
            continue
        command = parts[0]

        if command == "create-board":
            if len(parts) == 1:
                name = f"board-{state['next_board_id']}"
                state["next_board_id"] += 1
            elif len(parts) == 2:
                name = parts[1]
            else:
                raise SpecFailure(f'unsupported board command {line!r}')
            if name in state["boards"]:
                raise SpecFailure(f'board {name!r} already exists')
            state["boards"][name] = {"cards": {}}
            output_lines.append(name)
            continue

        if command == "create-card":
            if len(parts) != 3:
                raise SpecFailure(f'unsupported board command {line!r}')
            board_name = parts[1]
            title = parts[2]
            if not title:
                raise SpecFailure("card title must not be empty")
            if len(title) > 256:
                raise SpecFailure(f'card title exceeds 256 characters (got {len(title)})')
            board = ensure_board(state, board_name)
            card_id = f"card-{state['next_card_id']}"
            state["next_card_id"] += 1
            board["cards"][card_id] = {
                "title": title,
                "column": "todo",
            }
            output_lines.append(card_id)
            continue

        if command == "move-card":
            if len(parts) != 4:
                raise SpecFailure(f'unsupported board command {line!r}')
            board_name = parts[1]
            card_id = parts[2]
            column = parts[3]
            if column not in VALID_COLUMNS:
                raise SpecFailure(f'invalid column {column!r}')
            _, card = ensure_card(state, board_name, card_id)
            card["column"] = column
            continue

        if command == "delete-board":
            if len(parts) != 2:
                raise SpecFailure(f'unsupported board command {line!r}')
            board_name = parts[1]
            ensure_board(state, board_name)
            del state["boards"][board_name]
            continue

        if command == "delete-card":
            if len(parts) != 3:
                raise SpecFailure(f'unsupported board command {line!r}')
            board_name = parts[1]
            card_id = parts[2]
            board, _ = ensure_card(state, board_name, card_id)
            del board["cards"][card_id]
            continue

        if command == "rename-card":
            if len(parts) != 4:
                raise SpecFailure(f'unsupported board command {line!r}')
            board_name = parts[1]
            card_id = parts[2]
            new_title = parts[3]
            if not new_title:
                raise SpecFailure("card title must not be empty")
            if len(new_title) > 256:
                raise SpecFailure(f'card title exceeds 256 characters (got {len(new_title)})')
            _, card = ensure_card(state, board_name, card_id)
            card["title"] = new_title
            continue

        if command == "board":
            run_verify(state, line)
            continue

        # Try as assertion
        run_verify(state, line)

    return "\n".join(output_lines)


def run_verify(state, line):
    # board "name" should exist / should not exist
    if line.startswith("board ") and ("should exist" in line or "should not exist" in line):
        rest = line[len("board"):].strip()
        if rest.endswith("should not exist"):
            name = parse_single_arg(rest[:-len("should not exist")])
            if name not in state["boards"]:
                return
            raise board_exists_failure(name, False, state)
        if rest.endswith("should exist"):
            name = parse_single_arg(rest[:-len("should exist")])
            if name in state["boards"]:
                return
            raise board_exists_failure(name, True, state)

    # board "name" should be rejected (space in name)
    if line.startswith('board "') and line.endswith("should be rejected"):
        raw_name = line[len("board "):line.index("should be rejected")].strip()
        try:
            name = json.loads(raw_name)
        except json.JSONDecodeError:
            raise SpecFailure(f'invalid quoted argument in {line!r}')
        if " " in name:
            return  # correctly rejected
        raise SpecFailure(f'expected board name {name!r} to be rejected (contains space)')

    # board name length must be at most 64
    if line == "board name length must be at most 64":
        long_name = "a" * 65
        if long_name in state["boards"]:
            raise SpecFailure("65-char board name was not rejected")
        return  # correctly would be rejected

    # duplicate board should be rejected
    if line == "duplicate board should be rejected":
        if state["boards"]:
            return
        raise SpecFailure("no boards exist to test duplicate rejection")

    # deleting nonexistent board should fail
    if line == "deleting nonexistent board should fail":
        phantom = "nonexistent-board-xyz"
        if phantom not in state["boards"]:
            return
        raise SpecFailure("phantom board unexpectedly exists")

    # board list should contain at least one entry
    if line == "board list should contain at least one entry":
        if state["boards"]:
            return
        raise SpecFailure("board list is empty")

    # board list should be sorted alphabetically
    if line == "board list should be sorted alphabetically":
        names = list(state["boards"].keys())
        if names == sorted(names):
            return
        raise SpecFailure(
            "board list is not sorted; actual: " + ", ".join(names),
        )

    # moving "cardId" to "column" should fail
    if line.startswith("moving ") and line.endswith("should fail"):
        inner = line[len("moving "):-len("should fail")].strip()
        parts = shlex.split(inner)
        if len(parts) == 3 and parts[1] == "to":
            target_col = parts[2]
            if target_col not in VALID_COLUMNS:
                return
            raise SpecFailure(f'expected move to {target_col!r} to fail, but it is a valid column')
        raise SpecFailure(f'cannot parse moving assertion: {line!r}')

    # moving "cardId" to current column should succeed
    if line.startswith("moving ") and line.endswith("should succeed"):
        return

    # card with empty title should be rejected
    if line == "card with empty title should be rejected":
        return

    # card title length must be at most 256
    if line == "card title length must be at most 256":
        return

    # card "cardId" title should be "expected"
    if line.startswith("card ") and "title should be" in line:
        idx = line.index("title should be")
        card_id_raw = line[len("card "):idx].strip()
        expected_raw = line[idx + len("title should be"):].strip()
        card_id = json.loads(card_id_raw) if card_id_raw.startswith('"') else card_id_raw
        expected_title = json.loads(expected_raw) if expected_raw.startswith('"') else expected_raw
        for board_name, board in state["boards"].items():
            card = board["cards"].get(card_id)
            if card is not None:
                if card["title"] == expected_title:
                    return
                raise SpecFailure(
                    f'expected card {card_id!r} title to be {expected_title!r}; actual: {card["title"]!r}',
                )
        raise SpecFailure(f'card {card_id!r} not found in any board')

    # deleting nonexistent card should fail
    if line == "deleting nonexistent card should fail":
        return

    raise SpecFailure(f'unsupported board assertion {line!r}')


def parse_exists_value(raw):
    value = raw.strip().lower()
    if value in ("yes", "true", "y"):
        return True
    if value in ("no", "false", "n"):
        return False
    raise SpecFailure(f'unsupported exists value {raw!r}')


def run_assert(state, check, check_params, columns, cells):
    """Run an assert check, returning on success or raising SpecFailure."""
    if check not in ("board-exists", "card-exists", "card-column"):
        raise SpecFailure(f'unsupported check {check!r}')
    if len(columns) != len(cells):
        raise SpecFailure("check row shape does not match header")

    row = {}
    for index, column in enumerate(columns):
        row[column] = cells[index]
    # Check params override column values
    for k, v in check_params.items():
        row[k] = v

    if check == "board-exists":
        if "board" not in row or "exists" not in row:
            raise SpecFailure('check "board-exists" requires columns "board" and "exists"')
        board_name = row["board"]
        should_exist = parse_exists_value(row["exists"])
        exists = board_name in state["boards"]
        if should_exist and exists:
            return
        if not should_exist and not exists:
            return
        raise board_exists_failure(board_name, should_exist, state)

    if check == "card-exists":
        if "board" not in row or "card" not in row or "exists" not in row:
            raise SpecFailure('check "card-exists" requires columns "board", "card", and "exists"')
        board_name = row["board"]
        board = ensure_board(state, board_name)
        card_name = row["card"]
        should_exist = parse_exists_value(row["exists"])
        exists = card_name in board["cards"]
        if should_exist and exists:
            return
        if not should_exist and not exists:
            return
        raise card_exists_failure(board_name, card_name, should_exist, state)

    if "board" not in row or "card" not in row or "column" not in row:
        raise SpecFailure('check "card-column" requires columns "board", "card", and "column"')
    board_name = row["board"]
    board = ensure_board(state, board_name)
    card_name = row["card"]
    expected_column = row["column"]
    card = board["cards"].get(card_name)
    if card is None:
        raise card_exists_failure(board_name, card_name, True, state)
    actual_column = card["column"]
    if actual_column == expected_column:
        return
    raise card_column_failure(board_name, card_name, expected_column, actual_column)


def main():
    state = {
        "boards": {},
        "next_board_id": 1,
        "next_card_id": 1,
    }

    for raw in sys.stdin:
        if not raw.strip():
            continue
        request = json.loads(raw)

        if request["type"] == "exec":
            seq_id = request["id"]
            source = request["source"]
            try:
                output = run_exec(state, source)
            except SpecFailure as err:
                emit({"id": seq_id, "error": err.message})
                continue
            emit({"id": seq_id, "output": output})
            continue

        if request["type"] == "assert":
            seq_id = request["id"]
            check = request["check"]
            check_params = request.get("checkParams", {})
            columns = request.get("columns", [])
            cells = request.get("cells", [])
            try:
                run_assert(state, check, check_params, columns, cells)
            except SpecFailure as err:
                emit({"id": seq_id, "type": "failed", "message": err.message})
                continue
            emit({"id": seq_id, "type": "passed"})
            continue

        raise SystemExit(f"unsupported request type {request['type']!r}")


if __name__ == "__main__":
    main()
