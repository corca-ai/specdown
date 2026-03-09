#!/usr/bin/env python3
"""Pocket Board CLI — a tiny kanban board for specdown examples."""

import json
import os
import sys

STATE_FILE = os.environ.get("BOARD_STATE", ".board-state.json")
VALID_COLUMNS = ("todo", "doing", "done")


def load():
    try:
        with open(STATE_FILE) as f:
            return json.load(f)
    except FileNotFoundError:
        return {"boards": {}, "next_board": 1, "next_card": 1}


def save(state):
    with open(STATE_FILE, "w") as f:
        json.dump(state, f)


def fail(msg):
    print(msg, file=sys.stderr)
    sys.exit(1)


def main():
    args = sys.argv[1:]
    if not args:
        fail("usage: board.py <command> [args...]")
    cmd = args[0]
    state = load()

    if cmd == "create-board":
        if len(args) == 1:
            name = f"board-{state['next_board']}"
            state["next_board"] += 1
        else:
            name = args[1]
        if " " in name:
            fail("board name must not contain spaces")
        if len(name) > 64:
            fail("board name exceeds 64 characters")
        if name in state["boards"]:
            fail(f"board {name!r} already exists")
        state["boards"][name] = {"cards": {}}
        save(state)
        print(name)

    elif cmd == "delete-board":
        name = args[1]
        if name not in state["boards"]:
            fail(f"board {name!r} does not exist")
        del state["boards"][name]
        save(state)

    elif cmd == "board-exists":
        name = args[1]
        print("yes" if name in state["boards"] else "no")

    elif cmd == "list-boards":
        for name in sorted(state["boards"]):
            print(name)

    elif cmd == "create-card":
        board_name, title = args[1], args[2]
        if board_name not in state["boards"]:
            fail(f"board {board_name!r} does not exist")
        if not title:
            fail("card title must not be empty")
        if len(title) > 256:
            fail("card title exceeds 256 characters")
        card_id = f"card-{state['next_card']}"
        state["next_card"] += 1
        state["boards"][board_name]["cards"][card_id] = {
            "title": title,
            "column": "todo",
        }
        save(state)
        print(card_id)

    elif cmd == "delete-card":
        board_name, card_id = args[1], args[2]
        board = state["boards"].get(board_name)
        if not board or card_id not in board["cards"]:
            fail(f"card {card_id!r} does not exist")
        del board["cards"][card_id]
        save(state)

    elif cmd == "move-card":
        board_name, card_id, column = args[1], args[2], args[3]
        if column not in VALID_COLUMNS:
            fail(f"invalid column {column!r}")
        board = state["boards"].get(board_name)
        if not board or card_id not in board["cards"]:
            fail(f"card {card_id!r} does not exist")
        board["cards"][card_id]["column"] = column
        save(state)

    elif cmd == "card-exists":
        board_name, card_id = args[1], args[2]
        board = state["boards"].get(board_name)
        print("yes" if board and card_id in board["cards"] else "no")

    elif cmd == "card-column":
        board_name, card_id = args[1], args[2]
        board = state["boards"].get(board_name)
        if not board or card_id not in board["cards"]:
            fail(f"card {card_id!r} does not exist")
        print(board["cards"][card_id]["column"])

    elif cmd == "card-title":
        board_name, card_id = args[1], args[2]
        board = state["boards"].get(board_name)
        if not board or card_id not in board["cards"]:
            fail(f"card {card_id!r} does not exist")
        print(board["cards"][card_id]["title"])

    elif cmd == "rename-card":
        board_name, card_id, new_title = args[1], args[2], args[3]
        if not new_title:
            fail("card title must not be empty")
        if len(new_title) > 256:
            fail("card title exceeds 256 characters")
        board = state["boards"].get(board_name)
        if not board or card_id not in board["cards"]:
            fail(f"card {card_id!r} does not exist")
        board["cards"][card_id]["title"] = new_title
        save(state)

    else:
        fail(f"unknown command {cmd!r}")


if __name__ == "__main__":
    main()
