#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[2] == "":
        print("Usage: join_cli <sep> <file>")
        return 2
    sep, path = argv[1], argv[2]
    try:
        with open(path, encoding="utf-8") as f:
            raw = f.read()
    except OSError as e:
        print(f"error={e}")
        return 2
    print(sep.join(raw.split("\n")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
