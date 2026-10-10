#!/usr/bin/env python3
import sys
import time


def main(argv: list[str]) -> int:
    if len(argv) < 2 or argv[1] == "":
        print("Usage: sleep_cli <ms>")
        return 2
    time.sleep(int(argv[1]) / 1000.0)
    print("OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
