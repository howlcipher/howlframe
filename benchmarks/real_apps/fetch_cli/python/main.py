#!/usr/bin/env python3
import sys
import urllib.error
import urllib.request


def main(argv: list[str]) -> int:
    if len(argv) < 2 or argv[1] == "":
        print("Usage: fetch_cli <url> [method]")
        return 2
    url = argv[1]
    method = argv[2] if len(argv) > 2 and argv[2] != "" else "GET"
    req = urllib.request.Request(url, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            body = resp.read().decode()
            print(f"status={resp.status}")
            print(body)
    except urllib.error.HTTPError as e:
        print(f"status={e.code}")
        print(e.read().decode())
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
