#!/usr/bin/env python3
import json
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 3 or argv[1] == "" or argv[2] == "":
        print("Usage: file_transform <in.csv> <out.jsonl>")
        return 2
    inp, outp = argv[1], argv[2]
    try:
        with open(inp, encoding="utf-8") as f:
            raw = f.read()
    except OSError as e:
        print(f"error={e}")
        return 2
    n = 0
    buf = ""
    for row in raw.split("\n"):
        if row == "":
            continue
        cols = row.split(",")
        if len(cols) >= 2:
            n += 1
            obj = {"c0": cols[0], "c1": cols[1]}
            buf += json.dumps(obj, separators=(",", ":")) + "\n"
    with open(outp, "w", encoding="utf-8") as f:
        f.write(buf)
    print(f"rows={n}")
    print(f"out={outp}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
