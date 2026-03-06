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
    info = case["info"]
    source = case["source"]

    for raw_line in source.splitlines():
        line = raw_line.strip()
        if not line:
            continue

        if info == "run:board":
            if not line.startswith("create-board"):
                raise ValueError(f'unsupported board command {line!r}')
            name = parse_single_arg(line[len("create-board"):])
            if name in state["boards"]:
                raise ValueError(f'board {name!r} already exists')
            state["boards"].add(name)
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


def handle_describe():
    emit({
        "type": "capabilities",
        "blocks": ["run:board", "verify:board"],
        "fixtures": [],
    })


def handle_run(cases):
    state = {"boards": set()}
    for case in cases:
        label = default_label(case["id"], case["info"])
        emit({
            "type": "caseStarted",
            "id": case["id"],
            "label": label,
        })

        try:
            run_case(state, case)
        except ValueError as err:
            emit({
                "type": "caseFailed",
                "id": case["id"],
                "label": label,
                "message": str(err),
            })
            continue

        emit({
            "type": "casePassed",
            "id": case["id"],
            "label": label,
        })


def main():
    raw = sys.stdin.readline()
    if not raw:
        return

    request = json.loads(raw)
    if request["type"] == "describe":
        handle_describe()
        return
    if request["type"] == "run":
        handle_run(request.get("cases", []))
        return

    raise SystemExit(f"unsupported request type {request['type']!r}")


if __name__ == "__main__":
    main()
