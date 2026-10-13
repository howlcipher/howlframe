"""Shared, implementation-independent acceptance harness for ticketdesk (Python 3 stdlib only).

Launcher contract (REQUIREMENTS.md R1.1):

    LAUNCHER GRANTS [APP_ARG...]

GRANTS is ONE argument (comma-separated --allow-caps list).  Runner flags travel in the
environment variable HF_RUN_FLAGS; TICKETDESK_* configuration travels in the environment.
The harness starts servers on free ports with temporary data directories, polls
GET /v1/health for readiness, stops with SIGTERM (or SIGKILL when a test says so) and
kills whatever is left on cleanup.
"""
import argparse
import contextlib
import http.client
import json
import os
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time

BASE_FLAGS = "--max-instructions 4000000000 --max-memory-bytes 4294967296 --max-call-depth 10000"
READY_TIMEOUT = 25
DEFAULT = object()
REQID_RE = re.compile(r"^[0-9a-f]{32}$")

TESTS = []   # (id, reqs, title, fn)


def test(tid, reqs, title):
    def deco(fn):
        TESTS.append((tid, list(reqs), title, fn))
        return fn
    return deco


def check(cond, msg, res=None):
    if not cond:
        raise AssertionError(msg + ("\n" + repr(res) if res is not None else ""))


class Resp:
    def __init__(self, status, headers, body, method="GET"):
        self.status, self.hlist, self.body, self.method = status, headers, body, method
        self.headers = {}
        for k, v in headers:
            self.headers.setdefault(k.lower(), v)

    def h(self, name):
        return self.headers.get(name.lower())

    @property
    def text(self):
        return self.body.decode("utf-8", "replace")

    @property
    def json(self):
        try:
            return json.loads(self.body.decode("utf-8"))
        except Exception:
            return None

    def __repr__(self):
        return "HTTP %s %s\n%s" % (self.status, self.hlist, self.text[:800])


class Result:
    def __init__(self, rc, out, err, hung):
        self.rc, self.out, self.err, self.hung = rc, out, err, hung

    @property
    def both(self):
        return self.out + "\n" + self.err

    def __repr__(self):
        return "rc=%r hung=%r\n--stdout--\n%s\n--stderr--\n%s" % (self.rc, self.hung, self.out[-1200:], self.err[-1200:])


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    p = s.getsockname()[1]
    s.close()
    return p


class Server:
    """One ticketdesk process configured through env/flags/grants."""

    def __init__(self, ctx, env=None, flags="", grants=None, data=DEFAULT, admin=DEFAULT, port=None,
                 data_dir=None, name="srv"):
        self.ctx, self.name = ctx, name
        self.dir = data_dir or ctx.tmp()
        self.data = os.path.join(self.dir, "ticketdesk.state") if data is DEFAULT else data
        self.admin = ("adm-" + os.urandom(16).hex()) if admin is DEFAULT else admin
        self.port = port or free_port()
        self.flags = flags
        self.env_extra = dict(env or {})
        self.grants = grants if grants is not None else self.exact_grants(os.path.dirname(self.data) if self.data else self.dir)
        self.proc = None
        self.out_path = os.path.join(self.dir, "stdout.%d.log" % id(self))
        self.err_path = os.path.join(self.dir, "stderr.%d.log" % id(self))
        self.secrets = []

    @staticmethod
    def exact_grants(d, omit=None):
        d = os.path.realpath(d)
        g = ["network", "database", "environment", "filesystem:read=" + d, "filesystem:write=" + d]
        return ",".join(x for x in g if not (omit and x.split("=")[0] == omit))

    def environment(self):
        e = {k: v for k, v in os.environ.items() if not k.startswith("TICKETDESK_")}
        e["HF_RUN_FLAGS"] = (BASE_FLAGS + " " + self.flags).strip()
        if self.admin is not None:
            e["TICKETDESK_ADMIN_TOKEN"] = self.admin
        if self.data is not None:
            e["TICKETDESK_DATA"] = self.data
        if self.port is not None:
            e["TICKETDESK_PORT"] = str(self.port)
        for k, v in self.env_extra.items():
            if v is None:
                e.pop(k, None)
            else:
                e[k] = v
        return e

    def spawn(self):
        self.out_f = open(self.out_path, "ab")
        self.err_f = open(self.err_path, "ab")
        self.proc = subprocess.Popen([self.ctx.launcher, self.grants], env=self.environment(),
                                     stdout=self.out_f, stderr=self.err_f, stdin=subprocess.DEVNULL,
                                     cwd=self.ctx.root, start_new_session=True)
        self.ctx.live.append(self)

    def start(self, ready=True):
        self.spawn()
        if ready:
            self.wait_ready()
        return self

    def wait_ready(self, timeout=READY_TIMEOUT):
        t0 = time.time()
        while time.time() - t0 < timeout:
            if self.proc.poll() is not None:
                raise AssertionError("server exited early rc=%r\n--stderr--\n%s" % (self.proc.returncode, self.stderr()[-1500:]))
            try:
                r = self.req("GET", "/v1/health", timeout=2)
                if r.status == 200:
                    return
            except Exception:
                pass
            time.sleep(0.1)
        raise AssertionError("server not ready within %ss\n--stderr--\n%s" % (timeout, self.stderr()[-1500:]))

    def wait_exit(self, timeout):
        try:
            self.proc.wait(timeout)
            return self.proc.returncode
        except subprocess.TimeoutExpired:
            return None

    def stop(self, sig=signal.SIGTERM, wait=5):
        """Signal the launcher process; returns exit code or None if it did not exit in time."""
        if self.proc is None or self.proc.poll() is not None:
            return self.proc.returncode if self.proc else None
        try:
            self.proc.send_signal(sig)
        except ProcessLookupError:
            pass
        rc = self.wait_exit(wait)
        if rc is None:
            self.kill()
        return rc

    def kill(self):
        if self.proc is not None:
            try:
                os.killpg(self.proc.pid, signal.SIGKILL)
            except (ProcessLookupError, PermissionError):
                pass
            try:
                self.proc.wait(5)
            except Exception:
                pass
        for f in (getattr(self, "out_f", None), getattr(self, "err_f", None)):
            if f:
                f.close()

    def restart(self, sig=signal.SIGTERM, **changes):
        self.stop(sig)
        self.kill()
        if "admin" in changes:
            self.admin = changes["admin"]
        if "env" in changes:
            self.env_extra = changes["env"]
        self.start()
        return self

    def stderr(self):
        try:
            with open(self.err_path, "rb") as f:
                return f.read().decode("utf-8", "replace")
        except OSError:
            return ""

    def stdout(self):
        try:
            with open(self.out_path, "rb") as f:
                return f.read().decode("utf-8", "replace")
        except OSError:
            return ""

    def state_blobs(self):
        """Raw bytes of every file under the data directory (the state), name -> bytes."""
        blobs = {}
        for root, _dirs, files in os.walk(self.dir):
            for fn in files:
                if fn.startswith("stdout.") or fn.startswith("stderr."):
                    continue
                p = os.path.join(root, fn)
                try:
                    with open(p, "rb") as f:
                        blobs[os.path.relpath(p, self.dir)] = f.read()
                except OSError:
                    pass
        return blobs

    # ------------------------------------------------------------- HTTP
    def req(self, method, path, token=None, body=None, headers=None, ctype=DEFAULT, auth=DEFAULT,
            timeout=15):
        """body: dict/list/etc -> JSON; bytes/str -> raw."""
        if isinstance(body, (bytes, bytearray)):
            data = bytes(body)
        elif isinstance(body, str):
            data = body.encode("utf-8")
        elif body is None:
            data = None
        else:
            data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        conn = http.client.HTTPConnection("127.0.0.1", self.port, timeout=timeout)
        try:
            conn.putrequest(method, path, skip_accept_encoding=True)
            if auth is DEFAULT:
                auth = ("Bearer " + token) if token else None
            if auth is not None:
                for a in (auth if isinstance(auth, list) else [auth]):
                    conn.putheader("Authorization", a)
            if ctype is DEFAULT:
                ctype = "application/json" if data is not None else None
            if ctype is not None:
                conn.putheader("Content-Type", ctype)
            for k, v in (headers or []):
                conn.putheader(k, v)
            if data is not None:
                conn.putheader("Content-Length", str(len(data)))
            conn.putheader("Connection", "close")
            try:
                conn.endheaders(data)
            except (BrokenPipeError, ConnectionResetError):
                pass   # server answered (e.g. 413) before reading the whole body: read its response
            r = conn.getresponse()
            payload = r.read() if method != "HEAD" else b""
            return Resp(r.status, r.getheaders(), payload, method)
        finally:
            conn.close()


class Ctx:
    def __init__(self, launcher):
        self.launcher = os.path.abspath(launcher)
        self.root = os.path.realpath(tempfile.mkdtemp(prefix="tdacc-"))
        self._n = 0
        self.live = []
        self.cached = {}

    def tmp(self):
        self._n += 1
        d = os.path.join(self.root, "t%03d" % self._n)
        os.makedirs(d)
        return os.path.realpath(d)

    def make(self, **kw):
        return Server(self, **kw)

    def cache(self, name, **kw):
        """A long-lived shared server (state shared between tests; tests must not assume emptiness)."""
        if name not in self.cached:
            s = Server(self, name=name, **kw)
            s.start()
            self.cached[name] = s
        return self.cached[name]

    @contextlib.contextmanager
    def fresh(self, **kw):
        s = Server(self, **kw)
        try:
            s.start()
            yield s
        finally:
            s.stop(wait=3)
            s.kill()

    def try_start(self, timeout=15, **kw):
        """Start a server expected to fail at startup; returns Result."""
        s = Server(self, **kw)
        s.spawn()
        rc = s.wait_exit(timeout)
        hung = rc is None
        if hung:
            s.kill()
        else:
            s.kill()
        return Result(rc, s.stdout(), s.stderr(), hung), s

    def cleanup(self):
        for s in self.live:
            s.kill()
        shutil.rmtree(self.root, ignore_errors=True)


def main(tests_loaded=True):
    ap = argparse.ArgumentParser(description="ticketdesk black-box acceptance suite")
    ap.add_argument("launcher", nargs="?", help="path to launcher (LAUNCHER GRANTS [ARGS...])")
    ap.add_argument("--list", action="store_true", help="list test ids and exit")
    ap.add_argument("--only", action="append", default=[], help="run only these test ids (prefix match)")
    ap.add_argument("-v", action="store_true", help="show failure details")
    a = ap.parse_args()
    if a.list:
        for tid, reqs, title, _ in TESTS:
            print("%s\t%s\t%s" % (tid, ",".join(reqs), title))
        return 0
    if not a.launcher or not os.access(a.launcher, os.X_OK):
        print("error: launcher path required and must be executable", file=sys.stderr)
        return 2
    ctx = Ctx(a.launcher)
    results = {}
    try:
        for tid, reqs, title, fn in TESTS:
            if a.only and not any(tid.startswith(o) for o in a.only):
                continue
            t0 = time.time()
            try:
                fn(ctx)
                ok, why = True, ""
            except AssertionError as e:
                ok, why = False, str(e)
            except Exception as e:  # crash or harness bug: FAIL
                ok, why = False, "%s: %s" % (type(e).__name__, e)
            results[tid] = (ok, reqs)
            print("%s %s [%s] %s (%.1fs)" % ("PASS" if ok else "FAIL", tid, ",".join(reqs), title, time.time() - t0), flush=True)
            if not ok and a.v:
                print("    " + why.replace("\n", "\n    "), flush=True)
    finally:
        ctx.cleanup()
    per_req = {}
    for tid, (ok, reqs) in results.items():
        for r in reqs:
            per_req[r] = per_req.get(r, True) and ok

    def key(r):
        m = re.match(r"R(\d+)", r)
        return int(m.group(1)) if m else 999
    print("\n== per-requirement ==")
    for r in sorted(per_req, key=key):
        print("%s %s" % ("PASS" if per_req[r] else "FAIL", r))
    bad = [t for t, (ok, _) in results.items() if not ok]
    if not results:
        print("error: no tests selected", file=sys.stderr)
        return 2
    print("\n%d tests, %d passed, %d failed" % (len(results), len(results) - len(bad), len(bad)))
    return 1 if bad else 0
