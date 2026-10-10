#!/usr/bin/env python3
import re
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] == "" or argv[2] == "":
        print("Usage: grep_cli <pattern> <file>")
        return 2
    pat, path = argv[1], argv[2]
    try:
        with open(path, encoding="utf-8") as f:
            rows = f.read().splitlines()
    except OSError as e:
        print(f"error={e}")
        return 2
    n = 0
    for row in rows:
        if re.search(pat, row):
            n += 1
            print(row)
    print(f"hits={n}")
    return 0 if n > 0 else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
