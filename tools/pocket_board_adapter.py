#!/usr/bin/env python3

import json
import sys


def emit(payload):
    sys.stdout.write(json.dumps(payload) + "\n")
    sys.stdout.flush()


def default_label(case_id, info):
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
        raise ValueError("missing board name")

    if raw.startswith('"'):
        try:
            value = json.loads(raw)
        except json.JSONDecodeError:
            raise ValueError(f'invalid quoted argument {raw!r}')
        if not value:
            raise ValueError("board name must not be empty")
        return value

    if " " in raw or "\t" in raw:
        raise ValueError("board name must be a single token or quoted string")
    return raw


def parse_assertion(line):
    if not line.startswith("board"):
        raise ValueError(f'unsupported board assertion {line!r}')

    rest = line[len("board"):].strip()
    if rest.endswith("should exist"):
        return parse_single_arg(rest[:-len("should exist")]), True
    if rest.endswith("should not exist"):
        return parse_single_arg(rest[:-len("should not exist")]), False
    raise ValueError(f'unsupported board assertion {line!r}')


def run_case(state, case):
    info = case["block"]
    source = case["source"]
    capture_names = case.get("captureNames", [])
    bindings = binding_items(case)
    produced_bindings = []

    for raw_line in source.splitlines():
        line = render_arg(raw_line.strip(), bindings)
        if not line:
            continue

        if info == "run:board":
            if line == "create-board":
                if not capture_names:
                    raise ValueError("missing board name")
                name = f"board-{state['next_board_id']}"
                state["next_board_id"] += 1
            else:
                if not line.startswith("create-board"):
                    raise ValueError(f'unsupported board command {line!r}')
                name = parse_single_arg(line[len("create-board"):])
            if name in state["boards"]:
                raise ValueError(f'board {name!r} already exists')
            state["boards"].add(name)
            for capture_name in capture_names:
                produced_bindings.append({
                    "name": capture_name,
                    "value": name,
                })
            continue

        if info == "verify:board":
            name, should_exist = parse_assertion(line)
            exists = name in state["boards"]
            if should_exist and exists:
                continue
            if not should_exist and not exists:
                continue

            boards = "[" + ", ".join(json.dumps(item) for item in sorted(state["boards"])) + "]"
            if should_exist:
                raise ValueError(f'expected board {name!r} to exist; actual boards: {boards}')
            raise ValueError(f'expected board {name!r} not to exist; actual boards: {boards}')

        raise ValueError(f'unsupported case info {info!r}')

    return produced_bindings


def handle_describe():
    emit({
        "type": "capabilities",
        "blocks": ["run:board", "verify:board"],
        "fixtures": [],
    })


def handle_run_case(state, case):
    label = default_label(case["id"], case["block"])
    emit({
        "type": "caseStarted",
        "id": case["id"],
        "label": label,
    })

    try:
        produced_bindings = run_case(state, case)
    except ValueError as err:
        emit({
            "type": "caseFailed",
            "id": case["id"],
            "label": label,
            "message": str(err),
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
        "boards": set(),
        "next_board_id": 1,
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

        raise SystemExit(f"unsupported request type {request['type']!r}")


if __name__ == "__main__":
    main()
