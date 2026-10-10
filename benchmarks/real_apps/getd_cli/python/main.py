#!/usr/bin/env python3
import json
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] == "" or argv[2] == "":
        print("Usage: getd_cli <file> <key> [default]")
        return 2
    path, key = argv[1], argv[2]
    default = argv[3] if len(argv) > 3 and argv[3] != "" else "MISSING"
    try:
        with open(path, encoding="utf-8") as f:
            obj = json.load(f)
    except (OSError, json.JSONDecodeError) as e:
        print(f"error={e}")
        return 2
    print(obj[key] if key in obj else default)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
