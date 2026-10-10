#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] == "" or argv[2] == "":
        print("Usage: starts_cli <prefix> <file>")
        return 2
    prefix, path = argv[1], argv[2]
    try:
        with open(path, encoding="utf-8") as f:
            s = f.read()
    except OSError as e:
        print(f"error={e}")
        return 2
    if s.startswith(prefix):
        print("YES")
        return 0
    print("NO")
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
