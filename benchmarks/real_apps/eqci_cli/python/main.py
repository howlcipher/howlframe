#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] == "" or argv[2] == "":
        print("Usage: eqci_cli <a> <b>")
        return 2
    if argv[1].lower() == argv[2].lower():
        print("YES")
        return 0
    print("NO")
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
