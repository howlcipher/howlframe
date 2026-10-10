#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] == "" or argv[2] == "":
        print("Usage: minmax_cli <a> <b>")
        return 2
    a, b = int(argv[1]), int(argv[2])
    print(f"min={min(a, b)}")
    print(f"max={max(a, b)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
