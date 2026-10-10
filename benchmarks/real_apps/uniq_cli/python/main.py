#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 2 or argv[1] == "":
        print("Usage: uniq_cli <file>")
        return 2
    path = argv[1]
    try:
        with open(path, encoding="utf-8") as f:
            raw = f.read()
    except OSError as e:
        print(f"error={e}")
        return 2
    seen = []
    for row in raw.split("\n"):
        if row != "" and row not in seen:
            seen.append(row)
            print(row)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
