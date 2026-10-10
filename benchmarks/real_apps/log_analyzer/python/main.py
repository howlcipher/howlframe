#!/usr/bin/env python3
import re
import sys


def main(argv: list[str]) -> int:
    if len(argv) < 2 or argv[1] == "":
        print("HowlFrame Log Analyzer")
        print("Usage: log_analyzer <log-file>")
        return 2
    log_file = argv[1]
    try:
        with open(log_file, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError as err:
        print("HowlFrame Log Analyzer")
        print("file=", log_file)
        print("error=", err)
        return 2

    lines = content.split("\n")
    errors = warnings = todos = 0
    for line in lines:
        if re.search(r"ERROR|FATAL", line):
            errors += 1
        elif re.search(r"WARN|WARNING", line):
            warnings += 1
        elif re.search(r"TODO|FIXME", line):
            todos += 1

    print("HowlFrame Log Analyzer")
    print("file=", log_file)
    print("lines=", len(lines))
    print("errors=", errors)
    print("warnings=", warnings)
    print("todo=", todos)
    if errors > 0:
        print("status=ATTENTION")
        return 1
    print("status=CLEAN")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
