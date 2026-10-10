#!/usr/bin/env python3
import os
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 2 or argv[1] == "":
        print("Usage: mkdir_cli <path>")
        return 2
    os.makedirs(argv[1], exist_ok=True)
    print("OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
