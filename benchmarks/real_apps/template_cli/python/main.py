#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 4 or argv[1] == "" or argv[2] == "" or argv[3] == "":
        print("Usage: template_cli <file> <key> <value>")
        return 2
    path, key, val = argv[1], argv[2], argv[3]
    try:
        with open(path, encoding="utf-8") as f:
            s = f.read()
    except OSError as e:
        print(f"error={e}")
        return 2
    needle = "{{" + key + "}}"
    parts = s.split(needle)
    if len(parts) < 2:
        print("NO PLACEHOLDER")
        return 1
    print(parts[0] + val + parts[1], end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
