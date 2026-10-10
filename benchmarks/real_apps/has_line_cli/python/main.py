#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] == "" or argv[2] == "":
        print("Usage: has_line_cli <line> <file>")
        return 2
    needle, path = argv[1], argv[2]
    try:
        with open(path, encoding="utf-8") as f:
            raw = f.read()
    except OSError as e:
        print(f"error={e}")
        return 2
    if needle in raw.split("\n"):
        print("FOUND")
        return 0
    print("MISSING")
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
