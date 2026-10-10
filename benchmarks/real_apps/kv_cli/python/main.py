#!/usr/bin/env python3
import sys


def main(argv: list[str]) -> int:
    store: dict[str, str] = {}
    args = argv[1:]
    i = 0
    while i < len(args):
        cmd = args[i]
        if cmd == "set":
            i += 1
            k = args[i]
            i += 1
            v = args[i]
            store[k] = v
            print("OK")
        elif cmd == "get":
            i += 1
            k = args[i]
            if k not in store:
                print("NOT FOUND")
            else:
                print(store[k])
        elif cmd == "delete":
            i += 1
            k = args[i]
            store.pop(k, None)
            print("OK")
        elif cmd == "increment":
            i += 1
            k = args[i]
            current = int(store[k]) if k in store else 0
            next_val = current + 1
            store[k] = str(next_val)
            print(next_val)
        else:
            print("UNKNOWN CMD:", cmd)
            return 1
        i += 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
