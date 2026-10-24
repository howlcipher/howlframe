"""Time a program on a fixed workload inside the scoring jail (protocol §6 Runtime).

One warm-up run, then N timed runs. Wall time is measured around each run; peak RSS comes from wait4 on the
direct child, so `./run` wrappers should exec their program (the empty programs do). Prints one JSON object.
"""

import argparse
import json
import os
import statistics
import subprocess
import sys
import time


def timed(argv, stdin_path, cwd, timeout):
    stdin = open(stdin_path, "rb") if stdin_path else subprocess.DEVNULL
    try:
        t0 = time.perf_counter()
        proc = subprocess.Popen(argv, stdin=stdin, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=cwd)
        deadline = time.monotonic() + timeout
        while True:
            pid, status, ru = os.wait4(proc.pid, os.WNOHANG)
            if pid:
                break
            if time.monotonic() > deadline:
                proc.kill()
                pid, status, ru = os.wait4(proc.pid, 0)
                return {"seconds": None, "max_rss_kb": ru.ru_maxrss, "exit": "timeout"}
            time.sleep(0.001)
        elapsed = time.perf_counter() - t0
        return {"seconds": elapsed, "max_rss_kb": ru.ru_maxrss, "exit": os.waitstatus_to_exitcode(status)}
    finally:
        if stdin_path:
            stdin.close()


def measure(argv, stdin_path=None, cwd=None, repeats=5, timeout=120):
    warm = timed(argv, stdin_path, cwd, timeout)
    runs = [timed(argv, stdin_path, cwd, timeout) for _ in range(repeats)]
    ok = [r["seconds"] for r in runs if r["seconds"] is not None and r["exit"] == 0]
    return {"warmup": warm, "runs": runs,
            "median_s": statistics.median(ok) if len(ok) == repeats else None,
            "max_rss_kb": max(r["max_rss_kb"] for r in runs),
            "all_succeeded": len(ok) == repeats}


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--app", required=True)
    p.add_argument("--args", default="[]", help="JSON list of arguments")
    p.add_argument("--stdin")
    p.add_argument("--cwd")
    p.add_argument("--repeats", type=int, default=5)
    p.add_argument("--timeout", type=float, default=120)
    a = p.parse_args(argv)
    print(json.dumps(measure([a.app, *json.loads(a.args)], a.stdin, a.cwd, a.repeats, a.timeout)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
