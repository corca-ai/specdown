#!/usr/bin/env python3

import json
import shlex
import sys


def emit(payload):
    sys.stdout.write(json.dumps(payload) + "\n")
    sys.stdout.flush()


class SpecFailure(Exception):
    def __init__(self, message, expected="", actual=""):
        super().__init__(message)
        self.message = message
        self.expected = expected
        self.actual = actual


def default_label(case):
    case_id = case["id"]
    info = case.get("block", "")
    if case.get("kind") == "tableRow":
        return ""
    heading_path = case_id.get("headingPath", [])
    if heading_path:
        return f"{info} @ {heading_path[-1]}"
    return info


def binding_items(case):
    return {item["name"]: item["value"] for item in case.get("bindings", [])}


def render_arg(raw, bindings):
    raw = raw.strip()
    for name, value in bindings.items():
        raw = raw.replace("${" + name + "}", value)
    return raw


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


def parse_assertion(line):
    if not line.startswith("board"):
        raise SpecFailure(f'unsupported board assertion {line!r}')

    rest = line[len("board"):].strip()
    if rest.endswith("should exist"):
        return parse_single_arg(rest[:-len("should exist")]), True
    if rest.endswith("should not exist"):
        return parse_single_arg(rest[:-len("should not exist")]), False
    raise SpecFailure(f'unsupported board assertion {line!r}')


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
            f'expected board {board_name!r} to exist',
            actual=actual,
        )
    return SpecFailure(
        f'expected board {board_name!r} not to exist',
        actual=actual,
    )


def card_exists_failure(board_name, card_name, should_exist, state):
    board = state["boards"].get(board_name)
    names = sorted(board["cards"]) if board else []
    actual = ", ".join(names) if names else "(none)"
    if should_exist:
        return SpecFailure(
            f'expected card {card_name!r} in board {board_name!r} to exist',
            actual=actual,
        )
    return SpecFailure(
        f'expected card {card_name!r} in board {board_name!r} not to exist',
        actual=actual,
    )


def card_column_failure(board_name, card_name, column, actual_column):
    return SpecFailure(
        f'expected column {column!r}, got {actual_column!r}',
        actual=actual_column,
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


def run_case(state, case):
    kind = case["kind"]
    info = case.get("block", "")
    source = case.get("source", "")
    fixture = case.get("fixture", "")
    capture_names = case.get("captureNames", [])
    bindings = binding_items(case)
    produced_bindings = []

    if kind == "tableRow":
        return run_table_row(state, fixture, case.get("columns", []), case.get("cells", []))

    for raw_line in source.splitlines():
        line = render_arg(raw_line.strip(), bindings)
        if not line:
            continue
        parts = parse_command(line)
        if not parts:
            continue
        command = parts[0]

        if info == "run:board":
            if command == "create-board":
                if len(parts) == 1:
                    if not capture_names:
                        raise SpecFailure("missing board name")
                    name = f"board-{state['next_board_id']}"
                    state["next_board_id"] += 1
                elif len(parts) == 2:
                    name = parts[1]
                else:
                    raise SpecFailure(f'unsupported board command {line!r}')
                if name in state["boards"]:
                    raise SpecFailure(f'board {name!r} already exists')
                state["boards"][name] = {"cards": {}}
                for capture_name in capture_names:
                    produced_bindings.append({
                        "name": capture_name,
                        "value": name,
                    })
                continue

            if command == "create-card":
                if len(parts) != 3:
                    raise SpecFailure(f'unsupported board command {line!r}')
                board_name = parts[1]
                title = parts[2]
                if not title:
                    raise SpecFailure("card title must not be empty")
                board = ensure_board(state, board_name)
                card_id = f"card-{state['next_card_id']}"
                state["next_card_id"] += 1
                board["cards"][card_id] = {
                    "title": title,
                    "column": "todo",
                }
                for capture_name in capture_names:
                    produced_bindings.append({
                        "name": capture_name,
                        "value": card_id,
                    })
                continue

            if command == "move-card":
                if len(parts) != 4:
                    raise SpecFailure(f'unsupported board command {line!r}')
                board_name = parts[1]
                card_id = parts[2]
                column = parts[3]
                if column not in ("todo", "doing", "done"):
                    raise SpecFailure(f'unsupported column {column!r}')
                board = ensure_board(state, board_name)
                card = board["cards"].get(card_id)
                if card is None:
                    raise card_exists_failure(board_name, card_id, True, state)
                card["column"] = column
                continue

            raise SpecFailure(f'unsupported board command {line!r}')

        if info == "verify:board":
            name, should_exist = parse_assertion(line)
            exists = name in state["boards"]
            if should_exist and exists:
                continue
            if not should_exist and not exists:
                continue
            raise board_exists_failure(name, should_exist, state)

        raise SpecFailure(f'unsupported case info {info!r}')

    return produced_bindings


def parse_exists_value(raw):
    value = raw.strip().lower()
    if value in ("yes", "true", "y", "예"):
        return True
    if value in ("no", "false", "n", "아니오"):
        return False
    raise SpecFailure(f'unsupported exists value {raw!r}')


def run_table_row(state, fixture, columns, cells):
    if fixture not in ("board-exists", "card-exists", "card-column"):
        raise SpecFailure(f'unsupported fixture {fixture!r}')
    if len(columns) != len(cells):
        raise SpecFailure("fixture row shape does not match header")

    row = {}
    for index, column in enumerate(columns):
        row[column] = cells[index]

    if fixture == "board-exists":
        if "board" not in row or "exists" not in row:
            raise SpecFailure('fixture "board-exists" requires columns "board" and "exists"')
        board_name = row["board"]
        should_exist = parse_exists_value(row["exists"])
        exists = board_name in state["boards"]
        if should_exist and exists:
            return []
        if not should_exist and not exists:
            return []
        raise board_exists_failure(board_name, should_exist, state)

    if fixture == "card-exists":
        if "board" not in row or "card" not in row or "exists" not in row:
            raise SpecFailure('fixture "card-exists" requires columns "board", "card", and "exists"')
        board_name = row["board"]
        board = ensure_board(state, board_name)
        card_name = row["card"]
        should_exist = parse_exists_value(row["exists"])
        exists = card_name in board["cards"]
        if should_exist and exists:
            return []
        if not should_exist and not exists:
            return []
        raise card_exists_failure(board_name, card_name, should_exist, state)

    if "board" not in row or "card" not in row or "column" not in row:
        raise SpecFailure('fixture "card-column" requires columns "board", "card", and "column"')
    board_name = row["board"]
    board = ensure_board(state, board_name)
    card_name = row["card"]
    expected_column = row["column"]
    card = board["cards"].get(card_name)
    if card is None:
        raise card_exists_failure(board_name, card_name, True, state)
    actual_column = card["column"]
    if actual_column == expected_column:
        return []
    raise card_column_failure(board_name, card_name, expected_column, actual_column)


def handle_describe():
    emit({
        "type": "capabilities",
        "blocks": ["run:board", "verify:board"],
        "fixtures": ["board-exists", "card-exists", "card-column"],
    })


def handle_run_case(state, case):
    label = default_label(case)
    emit({
        "type": "caseStarted",
        "id": case["id"],
        "label": label,
    })

    try:
        produced_bindings = run_case(state, case)
    except SpecFailure as err:
        emit({
            "type": "caseFailed",
            "id": case["id"],
            "label": label,
            "message": err.message,
            "expected": err.expected,
            "actual": err.actual,
        })
        return

    emit({
        "type": "casePassed",
        "id": case["id"],
        "label": label,
        "bindings": produced_bindings,
    })


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
        if request["type"] == "describe":
            handle_describe()
            continue
        if request["type"] == "runCase":
            handle_run_case(state, request["case"])
            continue
        if request["type"] in ("setup", "teardown"):
            continue

        raise SystemExit(f"unsupported request type {request['type']!r}")


if __name__ == "__main__":
    main()
