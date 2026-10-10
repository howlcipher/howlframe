#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] == "" or argv[2] == "":
        print("Usage: cut_cli <index> <file>")
        return 2
    idx, path = int(argv[1]), argv[2]
    try:
        with open(path, encoding="utf-8") as f:
            raw = f.read()
    except OSError as e:
        print(f"error={e}")
        return 2
    for row in raw.split("\n"):
        if row == "":
            continue
        cols = row.split(",")
        if len(cols) > idx:
            print(cols[idx])
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
