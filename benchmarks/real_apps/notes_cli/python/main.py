#!/usr/bin/env python3
import json
import os
import sys


def path() -> str:
    uri = os.environ.get("NOTES_STORE", "file://notes.store")
    return uri[7:] if uri.startswith("file://") else uri


def load() -> dict:
    p = path()
    if not os.path.exists(p):
        return {}
    with open(p, encoding="utf-8") as f:
        return json.load(f)


def save(store: dict) -> None:
    with open(path(), "w", encoding="utf-8") as f:
        json.dump(store, f)


def main(argv: list[str]) -> int:
    store = load()
    args = argv[1:]
    i = 0
    while i < len(args):
        cmd = args[i]
        if cmd == "add":
            i += 1
            title = args[i]
            n = int(store.get("next_id", "1"))
            store[str(n)] = title
            store["next_id"] = str(n + 1)
            print(f"ADDED {n} {title}")
        elif cmd == "list":
            for k, v in store.items():
                if k != "next_id":
                    print(f"{k} {v}")
        elif cmd == "get":
            i += 1
            k = args[i]
            if k not in store:
                print("NOT FOUND")
                return 1
            print(f"{k} {store[k]}")
        elif cmd == "delete":
            i += 1
            k = args[i]
            if k not in store:
                print("NOT FOUND")
                return 1
            del store[k]
            print(f"DELETED {k}")
        else:
            print("UNKNOWN")
            return 1
        i += 1
    save(store)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
