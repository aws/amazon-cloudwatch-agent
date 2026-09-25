#!/usr/bin/env python3

import argparse
import hashlib
import json
from collections import OrderedDict
from pathlib import Path
from typing import Any


def _group_key(row: dict[str, Any]) -> tuple[str, str, bool]:
    return (
        str(row.get("test_dir", "")),
        str(row.get("terraform_dir", "")),
        bool(row.get("wip", False)),
    )


def _pair_name(cases: list[dict[str, Any]]) -> str:
    names = [str(case.get("testName") or case.get("test_dir") or "unnamed") for case in cases]
    return " + ".join(names)[:240]


def pair_matrix(rows: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Pair similar test cases while preserving every input matrix row exactly once."""
    grouped: OrderedDict[tuple[str, str, bool], list[dict[str, Any]]] = OrderedDict()
    for row in rows:
        if not isinstance(row, dict):
            raise ValueError("every matrix entry must be a JSON object")
        grouped.setdefault(_group_key(row), []).append(row)

    case_pairs: list[list[dict[str, Any]]] = []
    leftovers: list[dict[str, Any]] = []
    for group in grouped.values():
        pairable_count = len(group) - (len(group) % 2)
        case_pairs.extend(group[index : index + 2] for index in range(0, pairable_count, 2))
        leftovers.extend(group[pairable_count:])

    case_pairs.extend(leftovers[index : index + 2] for index in range(0, len(leftovers), 2))

    pairs = []
    for index, cases in enumerate(case_pairs):
        identity = "\n".join(
            str(case.get("testName") or case.get("test_dir") or index) for case in cases
        )
        digest = hashlib.sha256(identity.encode("utf-8")).hexdigest()[:10]
        pairs.append(
            {
                "pairId": f"{index:03d}-{digest}",
                "pairName": _pair_name(cases),
                "cases": cases,
            }
        )
    return pairs


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Convert an integration-test matrix into pairs of isolated cases."
    )
    parser.add_argument("input", type=Path, help="Input JSON matrix")
    parser.add_argument("output", type=Path, help="Output JSON pair matrix")
    args = parser.parse_args()

    with args.input.open(encoding="utf-8") as input_file:
        rows = json.load(input_file)
    if not isinstance(rows, list):
        raise ValueError("matrix must be a JSON array")

    pairs = pair_matrix(rows)
    with args.output.open("w", encoding="utf-8") as output_file:
        json.dump(pairs, output_file, separators=(",", ":"))
        output_file.write("\n")

    print(f"Paired {len(rows)} cases into {len(pairs)} jobs")


if __name__ == "__main__":
    main()
