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
import tempfile
import time


def timed(argv, stdin_path, cwd, timeout, stderr=subprocess.DEVNULL, stdout=subprocess.DEVNULL):
    stdin = open(stdin_path, "rb") if stdin_path else subprocess.DEVNULL
    try:
        t0 = time.perf_counter()
        proc = subprocess.Popen(argv, stdin=stdin, stdout=stdout, stderr=stderr, cwd=cwd)
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


def once(argv, stdin_path=None, cwd=None, timeout=600):
    """A single run that keeps the ends of stderr and stdout, to classify behaviour on an oversized workload."""
    with tempfile.TemporaryFile() as err, tempfile.TemporaryFile() as out:
        r = timed(argv, stdin_path, cwd, timeout, stderr=err, stdout=out)
        for name, f in (("stderr_tail", err), ("stdout_tail", out)):
            f.seek(0, os.SEEK_END)
            f.seek(max(0, f.tell() - 2000))
            r[name] = f.read().decode(errors="replace")
    return r


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--app", required=True)
    p.add_argument("--args", default="[]", help="JSON list of arguments")
    p.add_argument("--stdin")
    p.add_argument("--cwd")
    p.add_argument("--repeats", type=int, default=5)
    p.add_argument("--timeout", type=float, default=120)
    p.add_argument("--once", action="store_true", help="one run, keeping the end of stderr")
    a = p.parse_args(argv)
    if a.once:
        print(json.dumps(once([a.app, *json.loads(a.args)], a.stdin, a.cwd, a.timeout)))
        return 0
    print(json.dumps(measure([a.app, *json.loads(a.args)], a.stdin, a.cwd, a.repeats, a.timeout)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
