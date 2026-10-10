#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 2 or argv[1] == "":
        print("Usage: wc_cli <file>")
        return 2
    path = argv[1]
    try:
        with open(path, "rb") as f:
            raw = f.read()
    except OSError as e:
        print(f"error={e}")
        return 2
    text = raw.decode("utf-8")
    rows = text.split("\n")
    lc = len(rows)
    wc = 0
    for row in rows:
        for w in row.split(" "):
            if w != "":
                wc += 1
    print(f"lines={lc}")
    print(f"words={wc}")
    print(f"bytes={len(raw)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
