#!/usr/bin/env python3
import json
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] == "" or argv[2] == "":
        print("Usage: json_get <file> <key>")
        return 2
    path, key = argv[1], argv[2]
    try:
        with open(path, encoding="utf-8") as f:
            obj = json.load(f)
    except OSError as e:
        print(f"error={e}")
        return 2
    except json.JSONDecodeError as e:
        print(f"error={e}")
        return 2
    if key not in obj:
        print("NOT FOUND")
        return 1
    print(obj[key])
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
