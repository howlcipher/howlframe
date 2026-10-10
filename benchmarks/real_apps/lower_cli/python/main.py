#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 2 or argv[1] == "":
        print("Usage: lower_cli <text>")
        return 2
    print(argv[1].lower())
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
