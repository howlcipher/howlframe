"""Shared, implementation-independent acceptance harness (Python 3 stdlib only).

Identical copy in each milestone1/<app>/tests/acceptance directory.
Launcher contract (see the app's REQUIREMENTS.md section 1):

    LAUNCHER GRANTS [APP_ARG...]

GRANTS is ONE argument holding the comma-separated runtime grant list (possibly
the empty string, meaning no grants).  Extra runner flags (resource limits) are
passed to the launcher through the environment variable HF_RUN_FLAGS.
"""
import argparse
import os
import shutil
import subprocess
import sys
import tempfile
import time

BASE_FLAGS = "--max-instructions 4000000000 --max-memory-bytes 4294967296 --max-call-depth 10000"
DEFAULT_TIMEOUT = 180


class Result:
    def __init__(self, rc, out, err, timed_out, elapsed):
        self.rc, self.out, self.err = rc, out, err
        self.timed_out, self.elapsed = timed_out, elapsed

    @property
    def both(self):
        return self.out + "\n" + self.err

    def __repr__(self):
        return "rc=%r timed_out=%r\n--stdout--\n%s\n--stderr--\n%s" % (
            self.rc, self.timed_out, self.out[-1500:], self.err[-1500:])


TESTS = []   # (id, reqs, title, fn)


def test(tid, reqs, title):
    def deco(fn):
        TESTS.append((tid, list(reqs), title, fn))
        return fn
    return deco


class Ctx:
    def __init__(self, launcher, fixtures):
        self.launcher = os.path.abspath(launcher)
        self.fixtures = fixtures
        self.root = tempfile.mkdtemp(prefix="m1acc-")
        self._n = 0

    def tmp(self):
        self._n += 1
        d = os.path.join(self.root, "t%03d" % self._n)
        os.makedirs(d)
        return os.path.realpath(d)

    def fixture(self, name):
        return os.path.join(self.fixtures, name)

    def stage(self, name, dest_dir, subs=None, dest_name=None):
        """Copy a fixture into dest_dir, substituting @KEY@ placeholders."""
        with open(self.fixture(name), encoding="utf-8", newline="") as f:
            text = f.read()
        for k, v in (subs or {}).items():
            text = text.replace("@%s@" % k, v)
        dest = os.path.join(dest_dir, dest_name or name)
        os.makedirs(os.path.dirname(dest), exist_ok=True)
        with open(dest, "w", encoding="utf-8", newline="") as f:
            f.write(text)
        return dest

    def run(self, grants, args, stdin="", env=None, flags=None, cwd=None,
            timeout=DEFAULT_TIMEOUT):
        e = dict(os.environ)
        e["HF_RUN_FLAGS"] = BASE_FLAGS if flags is None else flags
        for k, v in (env or {}).items():
            if v is None:
                e.pop(k, None)
            else:
                e[k] = v
        t0 = time.time()
        try:
            p = subprocess.run([self.launcher, grants] + list(args),
                               input=stdin.encode("utf-8") if isinstance(stdin, str) else stdin,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               env=e, cwd=cwd or self.root, timeout=timeout)
            return Result(p.returncode, p.stdout.decode("utf-8", "replace"),
                          p.stderr.decode("utf-8", "replace"), False, time.time() - t0)
        except subprocess.TimeoutExpired as x:
            return Result(None, (x.stdout or b"").decode("utf-8", "replace"),
                          (x.stderr or b"").decode("utf-8", "replace"), True, time.time() - t0)

    def cleanup(self):
        shutil.rmtree(self.root, ignore_errors=True)


# ---- assertions -----------------------------------------------------------
def check(cond, msg, res=None):
    if not cond:
        raise AssertionError(msg + ("\n" + repr(res) if res is not None else ""))


def rc_is(res, want):
    check(not res.timed_out, "timed out (hang)", res)
    check(res.rc == want, "expected exit %r, got %r" % (want, res.rc), res)


def no_runtime_leak(res):
    """A clean failure never shows raw runtime error objects or host tracebacks."""
    for needle in ('"phase":', "Traceback", "panic:", "goroutine "):
        check(needle not in res.both, "raw runtime/host error leaked (%s)" % needle, res)


def clean_denial(res, tool):
    rc_is(res, 77)
    check("capability denied" in res.err.lower(), "stderr lacks 'capability denied'", res)
    check(res.err.startswith(tool + ": ") or ("\n" + tool + ": ") in res.err,
          "stderr diagnostics must be prefixed '%s: '" % tool, res)
    no_runtime_leak(res)


def main(argv=None, fixtures_dir=None, extra_args=None):
    ap = argparse.ArgumentParser(description="Black-box acceptance suite")
    ap.add_argument("launcher", nargs="?", help="path to launcher (LAUNCHER GRANTS ARGS...)")
    ap.add_argument("--list", action="store_true", help="list test ids and exit")
    ap.add_argument("--only", action="append", default=[], help="run only these test ids (prefix match)")
    ap.add_argument("-v", action="store_true", help="show failure details")
    a = ap.parse_args(argv)
    if a.list:
        for tid, reqs, title, _ in TESTS:
            print("%s\t%s\t%s" % (tid, ",".join(reqs), title))
        return 0
    if not a.launcher or not os.access(a.launcher, os.X_OK):
        print("error: launcher path required and must be executable", file=sys.stderr)
        return 2
    ctx = Ctx(a.launcher, fixtures_dir)
    results = {}
    try:
        for tid, reqs, title, fn in TESTS:
            if a.only and not any(tid.startswith(o) for o in a.only):
                continue
            try:
                fn(ctx)
                ok, why = True, ""
            except AssertionError as e:
                ok, why = False, str(e)
            except Exception as e:  # harness/test bug or crash: count as FAIL
                ok, why = False, "%s: %s" % (type(e).__name__, e)
            results[tid] = (ok, reqs)
            print("%s %s [%s] %s" % ("PASS" if ok else "FAIL", tid, ",".join(reqs), title))
            if not ok and a.v:
                print("    " + why.replace("\n", "\n    "))
    finally:
        ctx.cleanup()
    per_req = {}
    for tid, (ok, reqs) in results.items():
        for r in reqs:
            per_req[r] = per_req.get(r, True) and ok
    print("\n== per-requirement ==")
    def key(r):
        return (r[0], int(r[1:])) if r[1:].isdigit() else (r[0], 0)
    for r in sorted(per_req, key=key):
        print("%s %s" % ("PASS" if per_req[r] else "FAIL", r))
    bad = [t for t, (ok, _) in results.items() if not ok]
    if not results:
        print("error: no tests selected", file=sys.stderr)
        return 2
    print("\n%d tests, %d passed, %d failed" % (len(results), len(results) - len(bad), len(bad)))
    return 1 if bad else 0
