#!/usr/bin/env python3
import os
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 2 or argv[1] == "":
        print("Usage: env_cli <name>")
        return 2
    v = os.environ.get(argv[1], "")
    if v == "":
        print("MISSING")
        return 1
    print(v)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
