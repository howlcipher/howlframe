#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] == "" or argv[2] == "":
        print("Usage: head_cli <n> <file>")
        return 2
    n, path = int(argv[1]), argv[2]
    try:
        with open(path, encoding="utf-8") as f:
            raw = f.read()
    except OSError as e:
        print(f"error={e}")
        return 2
    i = 0
    for row in raw.split("\n"):
        if i < n:
            print(row)
            i += 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
