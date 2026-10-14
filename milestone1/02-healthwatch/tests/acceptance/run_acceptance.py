#!/usr/bin/env python3
"""Black-box acceptance suite for healthwatch (milestone1/02-healthwatch).

Usage:  run_acceptance.py LAUNCHER        run all tests
        run_acceptance.py --list          list test ids (no launcher needed)
        run_acceptance.py LAUNCHER --only H-R7 -v

Launcher contract: LAUNCHER GRANTS [APP_ARG...]   (REQUIREMENTS.md 1.2)
The suite starts local stdlib HTTP servers on 127.0.0.1 (no external network).
Python 3 standard library only.  Implementation independent.
"""
import http.server
import json
import os
import socket
import sys
import threading
import time
import urllib.parse

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from harness import (test, check, rc_is, no_runtime_leak, clean_denial, main, BASE_FLAGS)  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
FIX = os.path.join(HERE, "fixtures")
T = "healthwatch"
BODY_JSON = {"status": "ok", "n": 3, "items": [{"id": 1}, {"id": 2}], "flag": True, "nothing": None, "f": 1.0}


# ------------------------------------------------------------------ test server
class Server:
    def __init__(self):
        self.hits = []           # (method, path-with-query, body)
        self.counters = {}
        self.up = True
        self.other_port = 0
        self.lock = threading.Lock()
        outer = self

        class H(http.server.BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.0"

            def log_message(self, *a):
                pass

            def handle_any(self):
                n = int(self.headers.get("Content-Length") or 0)
                body = self.rfile.read(n).decode("utf-8", "replace") if n else ""
                with outer.lock:
                    outer.hits.append((self.command, self.path, body))
                u = urllib.parse.urlparse(self.path)
                p, q = u.path, urllib.parse.parse_qs(u.query)
                code, ctype, data, extra = 200, "text/plain", b"OK healthy", {}
                if p == "/json":
                    ctype, data = "application/json", json.dumps(BODY_JSON).encode()
                elif p.startswith("/status/"):
                    code, data = int(p.split("/")[2]), ("status %s" % p.split("/")[2]).encode()
                elif p.startswith("/flaky/"):
                    with outer.lock:
                        k = outer.counters.get(p, 0) + 1
                        outer.counters[p] = k
                    if k <= int(q.get("fail", ["0"])[0]):
                        code, data = 503, b"flaky down"
                elif p == "/toggle":
                    if not outer.up:
                        code, data = 503, b"down"
                elif p == "/redirect":
                    code, extra = 302, {"Location": "/ok"}
                elif p == "/redirect-out":
                    code, extra = 302, {"Location": "http://127.0.0.1:%d/ok" % outer.other_port}
                elif p == "/big":
                    data = b"x" * 100000
                elif p == "/slow":
                    time.sleep(6)
                elif p == "/text":
                    data = b"plain text, not json"
                elif p == "/echo":
                    ctype = "application/json"
                    data = json.dumps({"method": self.command, "path": self.path, "body": body}).encode()
                self.send_response(code)
                self.send_header("Content-Type", ctype)
                self.send_header("Content-Length", str(len(data)))
                for k, v in extra.items():
                    self.send_header(k, v)
                self.end_headers()
                if self.command != "HEAD":
                    try:
                        self.wfile.write(data)
                    except OSError:
                        pass

            do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = do_HEAD = handle_any

        self.httpd = http.server.ThreadingHTTPServer(("127.0.0.1", 0), H)
        self.httpd.daemon_threads = True
        self.port = self.httpd.server_address[1]
        threading.Thread(target=self.httpd.serve_forever, daemon=True).start()

    def stop(self):
        self.httpd.shutdown()
        self.httpd.server_close()

    def count(self, prefix=""):
        with self.lock:
            return len([h for h in self.hits if h[1].startswith(prefix)])

    def url(self, path):
        return "http://127.0.0.1:%d%s" % (self.port, path)


class with_servers:
    def __init__(self, n=1):
        self.n = n

    def __enter__(self):
        self.s = [Server() for _ in range(self.n)]
        if self.n > 1:
            self.s[0].other_port = self.s[1].port
        return self.s if self.n > 1 else self.s[0]

    def __exit__(self, *a):
        for s in self.s:
            s.stop()


def closed_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    p = s.getsockname()[1]
    s.close()
    return p


# ------------------------------------------------------------------ helpers
def put(d, name, text):
    p = os.path.join(d, name)
    with open(p, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    return p


def fxtext(name, **subs):
    with open(os.path.join(FIX, name), encoding="utf-8", newline="") as f:
        t = f.read()
    for k, v in subs.items():
        t = t.replace("@%s@" % k, str(v))
    return t


def rd(d):
    return "filesystem:read=" + d


def rw(d):
    return "filesystem:read=%s,filesystem:write=%s" % (d, d)


def net(*ports):
    return ",".join("network:fetch=127.0.0.1:%d" % p for p in ports)


def join(*gs):
    return ",".join(g for g in gs if g)


def cfg(checks, defaults=None, version=1):
    o = {"version": version}
    if defaults is not None:
        o["defaults"] = defaults
    o["checks"] = checks
    return json.dumps(o)


def chk(name, url, **kw):
    d = {"name": name, "url": url}
    d.update(kw)
    return d


def parse_text(res):
    """-> (results, events, summary) from text output."""
    results, events, summary = [], [], None
    for ln in res.out.splitlines():
        f = ln.split("\t")
        if f[0] in ("PASS", "FAIL", "ERROR", "DENIED"):
            check(len(f) == 5, "bad result line %r" % ln, res)
            results.append({"status": f[0], "name": f[1], "http_status": None if f[2] == "-" else int(f[2]),
                            "attempts": int(f[3]), "reason": f[4]})
        elif f[0] == "EVENT":
            check(len(f) == 3, "bad event line %r" % ln, res)
            events.append((f[1], f[2]))
        elif f[0] == "SUMMARY":
            summary = dict(x.split("=") for x in f[1:])
        else:
            check(False, "unexpected line in text output: %r" % ln, res)
    check(summary is not None, "missing SUMMARY line", res)
    return results, events, {k: int(v) for k, v in summary.items()}


def by_name(results):
    return {r["name"]: r for r in results}


def no_leak(res, extra=()):
    for n in ("127.0.0.1", "http://", "OK healthy") + tuple(extra):
        check(n not in res.out and n not in res.err, "output leaks %r" % n, res)


def err_prefixed(res):
    for ln in res.err.splitlines():
        check(ln.startswith(T + ": "), "stderr line without prefix: %r" % ln, res)


def fail_clean(res, rc):
    rc_is(res, rc)
    check(res.out == "", "stdout must be empty", res)
    check(res.err.startswith(T + ": "), "stderr prefix", res)
    err_prefixed(res)
    no_runtime_leak(res)


def state_of(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


# ------------------------------------------------------------------ R1 CLI shell
@test("H-R1-version-help", ["R1"], "version string; help usage text")
def _(c):
    r = c.run("", ["version"])
    rc_is(r, 0)
    check(r.out == "healthwatch 1.0.0\n", "version", r)
    for a in ("help", "--help", "-h"):
        r = c.run("", [a])
        rc_is(r, 0)
        check(r.out.startswith("usage: healthwatch"), "usage", r)


@test("H-R1-usage-errors", ["R1", "R14"], "usage errors: exit 2, no stdout, no I/O, no network, no state")
def _(c):
    d = c.tmp()
    with with_servers() as s:
        conf = put(d, "c.json", cfg([chk("a", s.url("/ok"))]))
        st = os.path.join(d, "state.json")
        g = join(rw(d), net(s.port))
        bad = [
            [], ["bogus"], ["run"], ["validate"], ["status"], ["run", conf, conf], ["version", "x"],
            ["run", "--bogus", "1", conf], ["run", "--format=json", conf], ["run", "--format", "yaml", conf],
            ["run", "--state", st, "--state", st, conf], ["run", conf, "--state"],
            ["validate", "--format", "json", conf], ["validate", "--state", st, conf],
            ["status", "--state", st, st], ["run", "--fail-if-down", conf], ["status", st, st],
        ]
        for a in bad:
            r = c.run(g, a)
            rc_is(r, 2)
            check(r.out == "", "stdout on usage error: %r" % a, r)
            check(r.err.startswith(T + ": "), "prefix: %r" % a, r)
        check(s.count() == 0, "usage errors must not touch the network")
        check(not os.path.exists(st), "usage errors must not create state")


# ------------------------------------------------------------------ R2 validation
BASE = {"name": "a", "url": "http://127.0.0.1:1/x"}


def cfgj(**kw):
    o = {"version": 1, "checks": [dict(BASE)]}
    o.update(kw)
    return json.dumps(o)


def chk_with(**kw):
    c = dict(BASE)
    c.update(kw)
    return json.dumps({"version": 1, "checks": [c]})


CONFIG_ERRORS = [
    ("", "$"), ("[]", "$"), ("{", "$"), ("null", "$"),
    ('{"checks":[%s]}' % json.dumps(BASE), "version"),
    (cfgj(version=2), "version"), (cfgj(version="1"), "version"), (cfgj(version=1.5), "version"),
    ('{"version":1}', "checks"), (json.dumps({"version": 1, "checks": []}), "checks"),
    (json.dumps({"version": 1, "checks": {"a": 1}}), "checks"),
    (json.dumps({"version": 1, "checks": [1]}), "checks[0]"),
    (cfgj(defaults={"url": "http://x"}), "defaults.url"), (cfgj(defaults={"method": "TRACE"}), "defaults.method"),
    (cfgj(defaults={"retries": 9}), "defaults.retries"), (cfgj(defaults=[]), "defaults"),
    (json.dumps({"version": 1, "checks": [{"url": "http://h/"}]}), "checks[0].name"),
    (json.dumps({"version": 1, "checks": [{"name": "a"}]}), "checks[0].url"),
    (chk_with(name="a b"), "checks[0].name"), (chk_with(name="a/b"), "checks[0].name"), (chk_with(name=""), "checks[0].name"),
    (chk_with(name="x" * 65), "checks[0].name"), (chk_with(name=7), "checks[0].name"),
    (json.dumps({"version": 1, "checks": [BASE, dict(BASE)]}), "checks[1].name"),
    (chk_with(url="ftp://h/x"), "checks[0].url"), (chk_with(url="127.0.0.1:1/x"), "checks[0].url"),
    (chk_with(url="http://"), "checks[0].url"), (chk_with(url=5), "checks[0].url"),
    (chk_with(url="http://h/${FOO}"), "checks[0].url"), (chk_with(url="http://h/${ENV:lower}"), "checks[0].url"),
    (chk_with(method="TRACE"), "checks[0].method"), (chk_with(method="get"), "checks[0].method"),
    (chk_with(body="x"), "checks[0].body"), (chk_with(method="POST", body=5), "checks[0].body"),
    (chk_with(method="HEAD", body_contains="x"), "checks[0].body_contains"),
    (chk_with(method="HEAD", json_assert=[{"path": "a", "exists": True}]), "checks[0].json_assert"),
    (chk_with(expect_status=99), "checks[0].expect_status"), (chk_with(expect_status=600), "checks[0].expect_status"),
    (chk_with(expect_status="200"), "checks[0].expect_status"), (chk_with(expect_status=[]), "checks[0].expect_status"),
    (chk_with(expect_status=[200, "x"]), "checks[0].expect_status"), (chk_with(expect_status=True), "checks[0].expect_status"),
    (chk_with(expect_status=200.5), "checks[0].expect_status"),
    (chk_with(body_contains=""), "checks[0].body_contains"), (chk_with(body_contains="a${ENV:X}"), "checks[0].body_contains"),
    (chk_with(json_assert=[]), "checks[0].json_assert"), (chk_with(json_assert={}), "checks[0].json_assert"),
    (chk_with(json_assert=[{"path": "a"}]), "checks[0].json_assert[0]"),
    (chk_with(json_assert=[{"path": "a", "equals": 1, "exists": True}]), "checks[0].json_assert[0]"),
    (chk_with(json_assert=[{"path": "ok", "exists": True}, {"path": "", "equals": 1}]), "checks[0].json_assert[1].path"),
    (chk_with(json_assert=[{"path": "a", "exists": "yes"}]), "checks[0].json_assert[0]"),
    (chk_with(json_assert=[{"path": "a", "equals": [1]}]), "checks[0].json_assert[0]"),
    (chk_with(retries=6), "checks[0].retries"), (chk_with(retries=-1), "checks[0].retries"),
    (chk_with(retries=1.5), "checks[0].retries"), (chk_with(retries="1"), "checks[0].retries"),
    (chk_with(retry_delay_ms=5001), "checks[0].retry_delay_ms"), (chk_with(retry_delay_ms=-1), "checks[0].retry_delay_ms"),
    (chk_with(foo=1), "checks[0].foo"), (cfgj(extra=1), "extra"),
    (json.dumps({"version": 1, "checks": [BASE, dict(BASE, name="b", url="gopher://x")]}), "checks[1].url"),
    # fixed precedence: version before checks, name before url, earlier check before later key problems
    (json.dumps({"checks": [{"name": "a b", "url": "ftp://x"}], "version": 2}), "version"),
    (json.dumps({"version": 1, "checks": [{"url": "ftp://x", "name": "a b"}]}), "checks[0].name"),
    (json.dumps({"version": 1, "zzz": 1, "checks": [{"name": "a", "url": "http://h", "retries": 9, "bad": 1}]}), "checks[0].retries"),
]


@test("H-R2-validate-ok", ["R2", "R12"], "validate accepts good configs; needs only file read; stdin works with no grants")
def _(c):
    d = c.tmp()
    conf = put(d, "c.json", fxtext("config_basic.json", PORT=1234))
    r = c.run(rd(d), ["validate", conf])
    rc_is(r, 0)
    check(r.out == "OK: 4 check(s)\n" and r.err == "", "validate output", r)
    r = c.run("", ["validate", "-"], stdin=fxtext("config_basic.json", PORT=1234))
    rc_is(r, 0)
    check(r.out == "OK: 4 check(s)\n", "validate stdin", r)
    full = cfg([chk("a", "https://example.com/x", method="POST", body="b ${ENV:SOME_VAR}", expect_status=[200, 201],
                    body_contains="x", json_assert=[{"path": "a.b", "equals": None}, {"path": "c", "exists": False}],
                    retries=5, retry_delay_ms=5000),
                chk("b-2.x_y", "http://h:8080/p?q=${ENV:A_1}", method="HEAD")],
               defaults={"method": "GET", "expect_status": 204, "retries": 1, "retry_delay_ms": 10})
    r = c.run("", ["validate", "-"], stdin=full, env={"SOME_VAR": None, "A_1": None})
    rc_is(r, 0)
    check(r.out == "OK: 2 check(s)\n", "full-featured config (no env needed for validate)", r)


@test("H-R2-validate-errors", ["R2", "R14"], "every config rule yields exit 65 with 'config: <path>:' and nothing else on stdout")
def _(c):
    for text, path in CONFIG_ERRORS:
        r = c.run("", ["validate", "-"], stdin=text)
        fail_clean(r, 65)
        want = "%s: config: %s: " % (T, path)
        check(r.err.splitlines()[0].startswith(want), "expected diagnostic starting %r for config %r" % (want, text[:120]), r)
        check(len(r.err.splitlines()) == 1, "exactly one error line expected", r)


@test("H-R2-run-validates-before-io", ["R2", "R7"], "run reports config errors before any request or state change")
def _(c):
    d = c.tmp()
    with with_servers() as s:
        st = os.path.join(d, "state.json")
        text = json.dumps({"version": 1, "checks": [chk("ok", s.url("/ok")), chk("bad", s.url("/ok"), retries=99)]})
        r = c.run(join(rw(d), net(s.port)), ["run", "--state", st, "-"], stdin=text)
        fail_clean(r, 65)
        check("config: checks[1].retries: " in r.err, "path", r)
        check(s.count() == 0 and not os.path.exists(st), "no request / no state on config error", r)


@test("H-R2-defaults-merge", ["R2", "R3", "R4"], "defaults are inherited and overridden per check")
def _(c):
    with with_servers() as s:
        text = cfg([chk("inherit", s.url("/echo"), body="b1"),
                    chk("override", s.url("/echo"), method="PUT", body="b2", expect_status=200),
                    chk("status", s.url("/status/202"), method="GET")],
                   defaults={"method": "POST", "expect_status": 202, "retries": 0})
        # inherit: POST /echo -> 200 but default expects 202 => FAIL; override: PUT expects 200 => PASS; status: GET inherits 202 => PASS
        r = c.run(net(s.port), ["run", "-"], stdin=text)
        rc_is(r, 1)
        res = by_name(parse_text(r)[0])
        check(res["inherit"]["status"] == "FAIL" and res["inherit"]["reason"] == "status_mismatch", "inherit", r)
        check(res["override"]["status"] == "PASS" and res["status"]["status"] == "PASS", "override/explicit", r)
        check(("POST", "/echo", "b1") in s.hits and ("PUT", "/echo", "b2") in s.hits and ("GET", "/status/202", "") in s.hits, "methods seen by server: %r" % s.hits, r)


# ------------------------------------------------------------------ R3 / R8 execution + output
@test("H-R3-basic-golden", ["R3", "R4", "R7", "R8", "R14"], "golden text report for the basic config with fresh state; exit 1")
def _(c):
    d = c.tmp()
    with with_servers() as s:
        conf = put(d, "c.json", fxtext("config_basic.json", PORT=s.port))
        st = os.path.join(d, "state.json")
        t0 = int(time.time())
        r = c.run(join(rw(d), net(s.port)), ["run", "--state", st, conf])
        t1 = int(time.time())
        rc_is(r, 1)
        check(r.out == fxtext("config_basic.expected.txt"), "report differs from golden", r)
        check(r.err == "", "stderr must be empty when nothing is denied", r)
        no_leak(r)
        check(s.count("/ok") == 2 and s.count("/json") == 1 and s.count("/status/503") == 1 and s.count("/redirect") == 1, "request counts: %r" % s.hits, r)
        stt = state_of(st)
        check(stt["version"] == 1 and set(stt["checks"]) == {"web", "api", "broken", "moved"}, "state keys", r)
        e = stt["checks"]["broken"]
        check(e["health"] == "down" and e["last_status"] == "FAIL" and e["last_http_status"] == 503 and e["consecutive_failures"] == 1, "broken entry: %r" % e, r)
        e = stt["checks"]["web"]
        check(e["health"] == "up" and e["last_status"] == "PASS" and e["last_http_status"] == 200 and e["consecutive_failures"] == 0, "web entry", r)
        for k, e in stt["checks"].items():
            check(isinstance(e["since"], int) and isinstance(e["last_run"], int) and t0 - 5 <= e["since"] <= t1 + 5
                  and t0 - 5 <= e["last_run"] <= t1 + 5, "timestamps for %s: %r" % (k, e), r)
        check(set(e) == {"health", "since", "last_run", "last_status", "last_http_status", "consecutive_failures"}, "state entry keys", r)


@test("H-R8-json-format", ["R8", "R3"], "JSON report shape")
def _(c):
    d = c.tmp()
    with with_servers() as s:
        conf = put(d, "c.json", fxtext("config_basic.json", PORT=s.port))
        r = c.run(join(rd(d), net(s.port)), ["run", "--format", "json", conf])
        rc_is(r, 1)
        j = json.loads(r.out)
        check(set(j) == {"results", "events", "summary"} and j["events"] == [], "keys/events (no --state => no events): %r" % j, r)
        check(j["results"][2] == {"name": "broken", "status": "FAIL", "http_status": 503, "attempts": 1, "reason": "status_mismatch"}, "result record", r)
        check(j["summary"] == {"pass": 3, "fail": 1, "error": 0, "denied": 0}, "summary", r)
        check(r.out.endswith("\n"), "trailing newline", r)
        r = c.run(join(rw(d), net(s.port)), ["run", "--format", "json", "--state", os.path.join(d, "s.json"), conf])
        j = json.loads(r.out)
        check(j["events"] == [{"kind": "new", "name": n} for n in ("web", "api", "broken", "moved")], "events", r)


@test("H-R3-no-state-no-events", ["R3", "R7", "R13"], "without --state: no events, no state files, only network grant needed (config on stdin)")
def _(c):
    d = c.tmp()
    with with_servers() as s:
        r = c.run(net(s.port), ["run", "-"], stdin=cfg([chk("a", s.url("/ok"))]), cwd=d)
        rc_is(r, 0)
        check(r.out == "PASS\ta\t200\t1\tok\nSUMMARY\tpass=1\tfail=0\terror=0\tdenied=0\n", "output", r)
        check(os.listdir(d) == [], "no files may be created", r)


# ------------------------------------------------------------------ R4 expectations
@test("H-R4-expectations", ["R4", "R3"], "status lists, body_contains, json_assert semantics and reasons")
def _(c):
    with with_servers() as s:
        U = s.url
        checks = [
            chk("status-list", U("/ok"), expect_status=[404, 200]),
            chk("status-404", U("/status/404"), expect_status=404),
            chk("status-mismatch", U("/ok"), expect_status=201),
            chk("body-ok", U("/ok"), body_contains="OK heal"),
            chk("body-case", U("/ok"), body_contains="HEALTHY"),
            chk("json-str", U("/json"), json_assert=[{"path": "status", "equals": "ok"}]),
            chk("json-num", U("/json"), json_assert=[{"path": "n", "equals": 3}, {"path": "f", "equals": 1}, {"path": "n", "equals": 3.0}]),
            chk("json-type", U("/json"), json_assert=[{"path": "n", "equals": "3"}]),
            chk("json-bool-null", U("/json"), json_assert=[{"path": "flag", "equals": True}, {"path": "nothing", "equals": None},
                                                         {"path": "nothing", "exists": True}]),
            chk("json-bool-type", U("/json"), json_assert=[{"path": "flag", "equals": 1}]),
            chk("json-index", U("/json"), json_assert=[{"path": "items.0.id", "equals": 1}, {"path": "items.1.id", "equals": 2}]),
            chk("json-missing-eq", U("/json"), json_assert=[{"path": "nope", "equals": 1}]),
            chk("json-not-exists", U("/json"), json_assert=[{"path": "nope", "exists": False}, {"path": "items.5", "exists": False},
                                                          {"path": "items.00.id", "exists": False}, {"path": "items.-1", "exists": False},
                                                          {"path": "status.x", "exists": False}]),
            chk("json-exists-fail", U("/json"), json_assert=[{"path": "status", "exists": False}]),
            chk("json-invalid", U("/text"), json_assert=[{"path": "a", "exists": True}]),
            chk("order-status-first", U("/status/500"), body_contains="zzz", json_assert=[{"path": "a", "exists": True}]),
            chk("order-body-before-json", U("/text"), body_contains="zzz", json_assert=[{"path": "a", "exists": True}]),
            chk("head", U("/ok"), method="HEAD"),
        ]
        r = c.run(net(s.port), ["run", "--format", "json", "-"], stdin=cfg(checks))
        rc_is(r, 1)
        res = {x["name"]: x for x in json.loads(r.out)["results"]}
        want = {
            "status-list": ("PASS", "ok"), "status-404": ("PASS", "ok"), "status-mismatch": ("FAIL", "status_mismatch"),
            "body-ok": ("PASS", "ok"), "body-case": ("FAIL", "body_missing"), "json-str": ("PASS", "ok"),
            "json-num": ("PASS", "ok"), "json-type": ("FAIL", "json_mismatch"), "json-bool-null": ("PASS", "ok"),
            "json-bool-type": ("FAIL", "json_mismatch"), "json-index": ("PASS", "ok"), "json-missing-eq": ("FAIL", "json_mismatch"),
            "json-not-exists": ("PASS", "ok"), "json-exists-fail": ("FAIL", "json_mismatch"), "json-invalid": ("FAIL", "json_invalid"),
            "order-status-first": ("FAIL", "status_mismatch"), "order-body-before-json": ("FAIL", "body_missing"), "head": ("PASS", "ok"),
        }
        for k, (st, why) in want.items():
            check((res[k]["status"], res[k]["reason"]) == (st, why), "%s: got %r want %r" % (k, res[k], (st, why)), r)
        check(res["status-404"]["http_status"] == 404 and res["head"]["http_status"] == 200, "http_status field", r)
        check(("HEAD", "/ok", "") in s.hits, "server must see HEAD", r)


@test("H-R4-methods-and-bodies", ["R4", "R3"], "method and body observed by the server are exactly the configured ones")
def _(c):
    with with_servers() as s:
        checks = []
        for m in ("POST", "PUT", "PATCH"):
            checks.append(chk(m.lower(), s.url("/echo"), method=m, body="payload-" + m,
                              json_assert=[{"path": "method", "equals": m}, {"path": "body", "equals": "payload-" + m}]))
        checks.append(chk("delete", s.url("/echo"), method="DELETE", json_assert=[{"path": "method", "equals": "DELETE"}, {"path": "body", "equals": ""}]))
        checks.append(chk("post-unicode", s.url("/echo"), method="POST", body='café "q" \\ end',
                          json_assert=[{"path": "body", "equals": 'café "q" \\ end'}]))
        r = c.run(net(s.port), ["run", "-"], stdin=cfg(checks))
        rc_is(r, 0)
        res, _e, summ = parse_text(r)
        check(summ["pass"] == 5, "all five must pass", r)


# ------------------------------------------------------------------ R5 retries
@test("H-R5-retries", ["R5", "R3"], "retry counts, attempts, stop at first PASS, request counts at the server")
def _(c):
    with with_servers() as s:
        dead = closed_port()
        checks = [
            chk("flaky2-r2", s.url("/flaky/a?fail=2"), retries=2),
            chk("flaky2-r1", s.url("/flaky/b?fail=2"), retries=1),
            chk("flaky0-r3", s.url("/flaky/c?fail=0"), retries=3),
            chk("always-default", s.url("/status/500")),
            chk("always-r4", s.url("/status/500"), retries=4),
            chk("refused-r2", "http://127.0.0.1:%d/x" % dead, retries=2),
            chk("flaky1-default", s.url("/flaky/d?fail=1")),
        ]
        r = c.run(net(s.port, dead), ["run", "--format", "json", "-"], stdin=cfg(checks, defaults={"retries": 1}))
        rc_is(r, 1)
        res = {x["name"]: x for x in json.loads(r.out)["results"]}
        # explicit per-check retries override defaults.retries=1; 'always-default' sets none => default 1 applies (2 attempts)
        got = {k: (v["status"], v["attempts"]) for k, v in res.items()}
        check(got["flaky2-r2"] == ("PASS", 3), got, r)
        check(got["flaky2-r1"] == ("FAIL", 2), got, r)
        check(got["flaky0-r3"] == ("PASS", 1), got, r)
        check(got["always-default"] == ("FAIL", 2), "default retries=1 applies: %r" % (got,), r)
        check(got["always-r4"] == ("FAIL", 5), got, r)
        check(got["refused-r2"] == ("ERROR", 3), got, r)
        check(got["flaky1-default"] == ("PASS", 2), got, r)
        check(res["refused-r2"]["reason"] == "request_failed" and res["refused-r2"]["http_status"] is None, "refused result", r)
        check(s.count("/flaky/a") == 3 and s.count("/flaky/b") == 2 and s.count("/flaky/c") == 1 and s.count("/status/500") == 7, "server-side request counts: %r" % s.hits, r)


@test("H-R5-retry-delay", ["R5"], "retry_delay_ms pauses between attempts")
def _(c):
    with with_servers() as s:
        t = time.time()
        r = c.run(net(s.port), ["run", "-"], stdin=cfg([chk("a", s.url("/status/500"), retries=2, retry_delay_ms=400)]))
        el = time.time() - t
        rc_is(r, 1)
        check(el >= 0.75, "two pauses of 400 ms expected, elapsed %.2fs" % el, r)
        t = time.time()
        r = c.run(net(s.port), ["run", "-"], stdin=cfg([chk("a", s.url("/ok"), retries=5, retry_delay_ms=5000)]))
        rc_is(r, 0)
        check(time.time() - t < 4.0, "no pause when the first attempt passes", r)


# ------------------------------------------------------------------ R6 resilience
@test("H-R6-resilience-and-size-limit", ["R6", "R11"], "refused / oversized / bad checks never stop the run; fetch-size limit => response_too_large")
def _(c):
    with with_servers() as s:
        dead = closed_port()
        checks = [chk("refused", "http://127.0.0.1:%d/x" % dead), chk("first", s.url("/ok")), chk("big", s.url("/big")),
                  chk("last", s.url("/ok"), body_contains="healthy")]
        r = c.run(net(s.port, dead), ["run", "--format", "json", "-"], stdin=cfg(checks), flags=BASE_FLAGS + " --max-fetch-bytes 20000")
        rc_is(r, 1)
        res = {x["name"]: x for x in json.loads(r.out)["results"]}
        check((res["refused"]["status"], res["refused"]["reason"]) == ("ERROR", "request_failed"), "refused", r)
        check((res["big"]["status"], res["big"]["reason"], res["big"]["http_status"]) == ("ERROR", "response_too_large", None), "big", r)
        check(res["first"]["status"] == "PASS" and res["last"]["status"] == "PASS", "later checks still ran", r)
        no_runtime_leak(r)
        r = c.run(net(s.port), ["run", "-"], stdin=cfg([chk("big", s.url("/big"))]))
        rc_is(r, 0)   # without the tiny limit the large body is fine


# ------------------------------------------------------------------ R7 state / events
@test("H-R7-state-lifecycle", ["R7", "R8"], "new/down/recovered/removed events, since & consecutive_failures bookkeeping across runs")
def _(c):
    d = c.tmp()
    st = os.path.join(d, "state.json")
    with with_servers() as s:
        g = join(rw(d), net(s.port))
        base = [chk("toggle", s.url("/toggle")), chk("stable", s.url("/ok"))]
        text = cfg(base)
        conf = put(d, "c.json", text)

        def run(conf_path=conf, fmt="json"):
            r = c.run(g, ["run", "--format", fmt, "--state", st, conf_path])
            return r, json.loads(r.out)
        r1, j1 = run()
        rc_is(r1, 0)
        check(j1["events"] == [{"kind": "new", "name": "toggle"}, {"kind": "new", "name": "stable"}], "run1 events", r1)
        s1 = state_of(st)
        time.sleep(1.2)
        r2, j2 = run()
        rc_is(r2, 0)
        check(j2["events"] == [], "run2 must have no events: %r" % j2["events"], r2)
        s2 = state_of(st)
        check(s2["checks"]["toggle"]["since"] == s1["checks"]["toggle"]["since"], "since must be kept while health is unchanged", r2)
        check(s2["checks"]["toggle"]["last_run"] > s1["checks"]["toggle"]["last_run"], "last_run must advance", r2)
        time.sleep(1.2)
        s.up = False
        r3, j3 = run()
        rc_is(r3, 1)
        check(j3["events"] == [{"kind": "down", "name": "toggle"}], "run3 events: %r" % j3["events"], r3)
        s3 = state_of(st)["checks"]["toggle"]
        check(s3["health"] == "down" and s3["consecutive_failures"] == 1 and s3["since"] > s1["checks"]["toggle"]["since"]
              and s3["last_status"] == "FAIL" and s3["last_http_status"] == 503, "down entry: %r" % s3, r3)
        check(state_of(st)["checks"]["stable"]["since"] == s1["checks"]["stable"]["since"], "stable since kept", r3)
        time.sleep(1.2)
        r4, j4 = run()
        s4 = state_of(st)["checks"]["toggle"]
        check(j4["events"] == [] and s4["consecutive_failures"] == 2 and s4["since"] == s3["since"], "run4 still down: %r" % s4, r4)
        run(); s5 = state_of(st)["checks"]["toggle"]
        check(s5["consecutive_failures"] == 3, "cf=3", r4)
        s.up = True
        time.sleep(1.2)
        r6, j6 = run()
        rc_is(r6, 0)
        s6 = state_of(st)["checks"]["toggle"]
        check(j6["events"] == [{"kind": "recovered", "name": "toggle"}], "recovered: %r" % j6["events"], r6)
        check(s6["health"] == "up" and s6["consecutive_failures"] == 0 and s6["since"] > s3["since"], "recovered entry: %r" % s6, r6)
        # config change: drop 'stable' and 'toggle', add two new ones => new events (config order) then removed (sorted by name)
        conf2 = put(d, "c2.json", cfg([chk("zz-new", s.url("/ok")), chk("aa-new", s.url("/status/500"))]))
        r7, j7 = run(conf2)
        rc_is(r7, 1)
        check(j7["events"] == [{"kind": "new", "name": "zz-new"}, {"kind": "new", "name": "aa-new"},
                               {"kind": "removed", "name": "stable"}, {"kind": "removed", "name": "toggle"}], "events: %r" % j7["events"], r7)
        check(set(state_of(st)["checks"]) == {"zz-new", "aa-new"}, "removed entries deleted from state", r7)
        # text format of events
        r8 = c.run(g, ["run", "--state", st, conf])
        ev = [ln for ln in r8.out.splitlines() if ln.startswith("EVENT")]
        check(ev == ["EVENT\tnew\ttoggle", "EVENT\tnew\tstable", "EVENT\tremoved\taa-new", "EVENT\tremoved\tzz-new"], "text events: %r" % ev, r8)


@test("H-R7-all-fail-first-run", ["R7"], "state is written even when every check fails; ERROR results recorded with null http status")
def _(c):
    d = c.tmp()
    dead = closed_port()
    st = os.path.join(d, "state.json")
    r = c.run(join(rw(d), net(dead)), ["run", "--state", st, "-"], stdin=cfg([chk("x", "http://127.0.0.1:%d/" % dead)]))
    rc_is(r, 1)
    e = state_of(st)["checks"]["x"]
    check((e["health"], e["last_status"], e["last_http_status"], e["consecutive_failures"]) == ("down", "ERROR", None, 1), "entry %r" % e, r)


@test("H-R7-state-corrupt", ["R7", "R2"], "invalid state files => exit 65 before any request, file left untouched")
def _(c):
    d = c.tmp()
    with with_servers() as s:
        conf = put(d, "c.json", cfg([chk("a", s.url("/ok"))]))
        g = join(rw(d), net(s.port))
        for i, line in enumerate(fxtext("state_corrupt_list.txt").splitlines()):
            st = put(d, "state%d.json" % i, line + "\n")
            before = open(st, "rb").read()
            for args in (["run", "--state", st, conf], ["status", st]):
                r = c.run(g, args)
                fail_clean(r, 65)
                check(r.err.startswith(T + ": state: "), "diagnostic must start 'healthwatch: state: ' : line %d" % i, r)
            check(open(st, "rb").read() == before, "corrupt state must stay untouched (line %d)" % i)
        check(s.count() == 0, "no requests may be sent when state is invalid")


@test("H-R7-preflight-denied", ["R7", "R12"], "state not readable/writable => exit 77 with ZERO requests sent")
def _(c):
    d = c.tmp()
    with with_servers() as s:
        conf = put(d, "c.json", cfg([chk("a", s.url("/ok"))]))
        st = os.path.join(d, "state.json")
        r = c.run(join(rd(d), net(s.port)), ["run", "--state", st, conf])
        clean_denial(r, T)
        check(s.count() == 0, "request sent although state could not be written", r)
        check(not os.path.exists(st), "state file must not appear", r)
        # existing state, write-only denied => content unchanged
        put(d, "state.json", open(os.path.join(FIX, "state_valid.json")).read())
        before = open(st, "rb").read()
        r = c.run(join(rd(d), net(s.port)), ["run", "--state", st, conf])
        clean_denial(r, T)
        check(open(st, "rb").read() == before and s.count() == 0, "state altered / request sent", r)
        # config on stdin, no filesystem grants at all => state read denied
        r = c.run(net(s.port), ["run", "--state", st, "-"], stdin=cfg([chk("a", s.url("/ok"))]))
        clean_denial(r, T)
        check(s.count() == 0, "request sent although state read was denied", r)
        # write-only grant cannot read an existing state
        r = c.run(join("filesystem:write=" + d, net(s.port)), ["run", "--state", st, "-"], stdin=cfg([chk("a", s.url("/ok"))]))
        clean_denial(r, T)
        check(s.count() == 0, "request sent", r)


@test("H-R7-denied-keeps-state", ["R7", "R12"], "DENIED checks leave existing state untouched, add nothing, emit no events")
def _(c):
    d = c.tmp()
    with with_servers(2) as (s, other):
        st = os.path.join(d, "state.json")
        g = join(rw(d), net(s.port))
        r = c.run(g, ["run", "--state", st, "-"], stdin=cfg([chk("keep", s.url("/ok"))]))
        rc_is(r, 0)
        before = state_of(st)
        text = cfg([chk("keep", other.url("/ok")), chk("brand-new", other.url("/ok")), chk("fine", s.url("/ok"))])
        r = c.run(g, ["run", "--format", "json", "--state", st, "-"], stdin=text)
        rc_is(r, 77)
        j = json.loads(r.out)
        check([x["status"] for x in j["results"]] == ["DENIED", "DENIED", "PASS"], "results %r" % j["results"], r)
        check(j["events"] == [{"kind": "new", "name": "fine"}], "events: %r" % j["events"], r)
        after = state_of(st)
        check(after["checks"]["keep"] == before["checks"]["keep"], "denied existing entry changed", r)
        check(set(after["checks"]) == {"keep", "fine"}, "denied new name must not be added", r)
        check(other.count() == 0, "ungranted server received requests", r)
        # a denied existing check is not reported as removed
        check(all(e["kind"] != "removed" for e in j["events"]), "denied != removed", r)


# ------------------------------------------------------------------ R9 status
@test("H-R9-status", ["R9"], "status command: sorting, formats, --fail-if-down, missing/corrupt")
def _(c):
    d = c.tmp()
    st = put(d, "state.json", fxtext("state_valid.json"))
    r = c.run(rd(d), ["status", st])
    rc_is(r, 0)
    check(r.out == fxtext("state_valid.status.txt") and r.err == "", "status text", r)
    r = c.run(rd(d), ["status", "--format", "json", st])
    rc_is(r, 0)
    j = json.loads(r.out)
    check([x["name"] for x in j["checks"]] == ["alpha", "mid.1", "zeta"], "sorted", r)
    check(j["checks"][0] == {"name": "alpha", "health": "down", "since": 1760000100, "last_run": 1760000300,
                              "last_status": "ERROR", "last_http_status": None, "consecutive_failures": 3}, "record", r)
    r = c.run(rd(d), ["status", "--fail-if-down", st])
    rc_is(r, 1)
    check(r.out == fxtext("state_valid.status.txt"), "output unchanged with --fail-if-down", r)
    up = put(d, "up.json", json.dumps({"version": 1, "checks": {"a": {"health": "up", "since": 1, "last_run": 2, "last_status": "PASS", "last_http_status": 200, "consecutive_failures": 0}}}))
    rc_is(c.run(rd(d), ["status", "--fail-if-down", up]), 0)
    empty = put(d, "empty.json", '{"version":1,"checks":{}}')
    r = c.run(rd(d), ["status", empty])
    rc_is(r, 0)
    check(r.out == "", "empty text", r)
    r = c.run(rd(d), ["status", "--format", "json", empty])
    check(json.loads(r.out) == {"checks": []}, "empty json", r)
    fail_clean(c.run(rd(d), ["status", os.path.join(d, "missing.json")]), 66)
    fail_clean(c.run(rd(d), ["status", put(d, "bad.json", "{")]), 65)
    clean_denial(c.run("", ["status", st]), T)
    clean_denial(c.run(rd(c.tmp()), ["status", st]), T)


# ------------------------------------------------------------------ R10 environment
@test("H-R10-env-interpolation", ["R10", "R14"], "${ENV:} expands in url/body, secrets never leak to stdout/stderr/state")
def _(c):
    d = c.tmp()
    secret = "s3cr3t-Tok_9f2"
    with with_servers() as s:
        text = cfg([chk("tok", s.url("/echo?token=${ENV:HW_TOKEN}"), json_assert=[{"path": "path", "equals": "/echo?token=" + secret}]),
                    chk("body", s.url("/echo"), method="POST", body="k=${ENV:HW_TOKEN};${ENV:HW_TOKEN}",
                        json_assert=[{"path": "body", "equals": "k=%s;%s" % (secret, secret)}])])
        conf = put(d, "c.json", text)
        st = os.path.join(d, "state.json")
        r = c.run(join(rw(d), net(s.port), "environment"), ["run", "--state", st, conf], env={"HW_TOKEN": secret})
        rc_is(r, 0)
        check(r.out.count("PASS") == 2, "both pass", r)
        for blob in (r.out, r.err, open(st).read()):
            check(secret not in blob, "secret leaked")
        no_leak(r)
        # unset variable => config error 65, nothing sent
        n0 = s.count()
        r = c.run(join(rw(d), net(s.port), "environment"), ["run", "--state", st, conf], env={"HW_TOKEN": None})
        fail_clean(r, 65)
        check("undefined environment variable HW_TOKEN" in r.err and r.err.startswith(T + ": config: "), "message", r)
        r2 = c.run(join(rw(d), net(s.port), "environment"), ["run", "--state", st, conf], env={"HW_TOKEN": ""})
        fail_clean(r2, 65)
        check(s.count() == n0, "no requests when expansion fails", r)
        # environment capability missing => 77 before any request and before state preflight
        st2 = os.path.join(d, "state2.json")
        r = c.run(join(rw(d), net(s.port)), ["run", "--state", st2, conf], env={"HW_TOKEN": secret})
        clean_denial(r, T)
        check("environment" in r.err, "denial must name 'environment'", r)
        check(s.count() == n0 and not os.path.exists(st2), "no request / no state when env is denied", r)
        check(secret not in r.out + r.err, "secret leaked on denial", r)
        # validate never needs the environment
        r = c.run(rd(d), ["validate", conf], env={"HW_TOKEN": None})
        rc_is(r, 0)
        # configs without references do not need the grant
        r = c.run(net(s.port), ["run", "-"], stdin=cfg([chk("a", s.url("/ok"))]), env={"HW_TOKEN": None})
        rc_is(r, 0)


# ------------------------------------------------------------------ R11 limits
@test("H-R11-check-limit", ["R11"], "more than 500 checks => exit 70, no requests")
def _(c):
    with with_servers() as s:
        many = [chk("c%d" % i, s.url("/ok")) for i in range(501)]
        r = c.run(net(s.port), ["run", "-"], stdin=cfg(many))
        fail_clean(r, 70)
        check("check limit exceeded (max 500)" in r.err and s.count() == 0, "limit message / no requests", r)
        r = c.run(net(s.port), ["run", "-"], stdin=cfg(many[:500]), timeout=170)
        rc_is(r, 0)
        check(s.count() == 500, "500 checks must all run", r)


@test("H-R11-deadline", ["R11", "R7"], "runner deadline stops a slow run promptly, state untouched")
def _(c):
    d = c.tmp()
    with with_servers() as s:
        st = put(d, "state.json", fxtext("state_valid.json"))
        conf = put(d, "c.json", cfg([chk("slow", s.url("/slow"))]))
        before = state_of(st)
        ctl = put(d, "ctl.json", cfg([chk("fast", s.url("/ok"))]))   # positive control under the same deadline
        r0 = c.run(join(rw(d), net(s.port)), ["run", ctl], flags=BASE_FLAGS + " --deadline 20s", timeout=60)
        rc_is(r0, 0)
        t = time.time()
        r = c.run(join(rw(d), net(s.port)), ["run", "--state", st, conf], flags=BASE_FLAGS + " --deadline 1s", timeout=60)
        check(not r.timed_out and time.time() - t < 12, "deadline not honored (hang)", r)
        check(r.rc not in (0, None), "must fail when the deadline expires", r)
        check(state_of(st) == before, "state must be unchanged after deadline stop", r)
        check("PASS" not in r.out, "no report", r)


@test("H-R11-instruction-limit", ["R11"], "tiny instruction ceiling stops the program with non-zero exit")
def _(c):
    d = c.tmp()
    conf = put(d, "c.json", fxtext("config_basic.json", PORT=1))
    rc_is(c.run(rd(d), ["validate", conf]), 0)   # positive control
    r = c.run(rd(d), ["validate", conf], flags="--max-instructions 300", timeout=60)
    check(not r.timed_out and r.rc not in (0, None) and "OK" not in r.out, "must stop under tiny ceiling", r)


# ------------------------------------------------------------------ R12 capabilities
@test("H-R12-network-denied", ["R12", "R3", "R14"], "no fetch grant: every check DENIED, full report, exit 77, zero requests, no retries")
def _(c):
    with with_servers() as s:
        checks = [chk("a", s.url("/ok"), retries=3), chk("b", s.url("/json"))]
        for grants in ("", "network", "network:fetch=127.0.0.1:%d" % (s.port + 1), "network:fetch=localhost:%d" % s.port):
            r = c.run(grants, ["run", "-"], stdin=cfg(checks))
            rc_is(r, 77)
            res, ev, summ = parse_text(r)
            check([(x["status"], x["attempts"], x["http_status"], x["reason"]) for x in res] == [("DENIED", 1, None, "capability_denied")] * 2,
                  "results %r (grants=%r)" % (res, grants), r)
            check(summ == {"pass": 0, "fail": 0, "error": 0, "denied": 2}, "summary", r)
            lines = r.err.splitlines()
            check(lines == ["healthwatch: capability denied: network (a)", "healthwatch: capability denied: network (b)"], "stderr: %r" % lines, r)
        check(s.count() == 0, "denied checks must send no request")


@test("H-R12-mixed-and-redirect", ["R12", "R3", "R6"], "per-host grants; DENIED outranks FAIL in the exit code; redirect hop to ungranted port is DENIED")
def _(c):
    with with_servers(2) as (a, b):
        checks = [chk("granted-ok", a.url("/ok")), chk("granted-fail", a.url("/status/500")), chk("other-host", b.url("/ok")),
                  chk("follows-granted-redirect", a.url("/redirect")), chk("redirect-out", a.url("/redirect-out"))]
        r = c.run(net(a.port), ["run", "--format", "json", "-"], stdin=cfg(checks))
        rc_is(r, 77)
        res = {x["name"]: x["status"] for x in json.loads(r.out)["results"]}
        check(res == {"granted-ok": "PASS", "granted-fail": "FAIL", "other-host": "DENIED", "follows-granted-redirect": "PASS",
                      "redirect-out": "DENIED"}, "statuses %r" % res, r)
        check(b.count() == 0, "ungranted server must never be contacted (including via redirect)", r)
        check(a.count("/redirect-out") == 1, "first hop is allowed", r)
        # with both granted the redirect-out check passes and the exit status is 1 (only FAIL left)
        r = c.run(net(a.port, b.port), ["run", "-"], stdin=cfg(checks))
        rc_is(r, 1)


@test("H-R12-fs-denied", ["R12", "R14"], "config read denied/out-of-scope => 77 and zero requests; missing config => 66")
def _(c):
    d = c.tmp()
    other = c.tmp()
    with with_servers() as s:
        conf = put(d, "c.json", cfg([chk("a", s.url("/ok"))]))
        clean_denial(c.run(net(s.port), ["run", conf]), T)
        clean_denial(c.run(net(s.port), ["validate", conf]), T)
        clean_denial(c.run(join(rd(other), net(s.port)), ["run", conf]), T)
        clean_denial(c.run(rd(other), ["validate", conf]), T)
        check(s.count() == 0, "no request after fs denial")
        fail_clean(c.run(rd(d), ["validate", os.path.join(d, "none.json")]), 66)
        fail_clean(c.run(join(rd(d), net(s.port)), ["run", os.path.join(d, "none.json")]), 66)


@test("H-R12-minimal-grants", ["R12", "R13"], "success with exactly: read dir + write dir + fetch grant (no process/database/network-serving)")
def _(c):
    d = c.tmp()
    with with_servers() as s:
        conf = put(d, "c.json", cfg([chk("a", s.url("/ok"))]))
        r = c.run("filesystem:read=%s,filesystem:write=%s,network:fetch=127.0.0.1:%d" % (d, d, s.port),
                  ["run", "--state", os.path.join(d, "s.json"), conf])
        rc_is(r, 0)
        check(r.out.startswith("PASS\ta\t200\t1\tok\n"), "output", r)


# ------------------------------------------------------------------ R14 hygiene
@test("H-R14-hygiene", ["R14"], "diagnostics prefixed; no URLs/hosts/bodies in any output; clean runs are silent on stderr")
def _(c):
    d = c.tmp()
    with with_servers() as s:
        text = cfg([chk("a", s.url("/ok?x=1")), chk("b", s.url("/status/500")), chk("c", "http://127.0.0.1:%d/z" % closed_port())])
        conf = put(d, "c.json", text)
        r = c.run(join(rw(d), net(s.port)), ["run", "--state", os.path.join(d, "s.json"), conf])
        rc_is(r, 77)   # 'c' is DENIED (port not granted)
        err_prefixed(r)
        no_leak(r, extra=("x=1", "status 500"))
        st_text = open(os.path.join(d, "s.json")).read()
        check("127.0.0.1" not in st_text and "://" not in st_text, "state must not contain URLs", r)
        r = c.run(join(rw(d), net(s.port)), ["run", "--format", "json", "-"], stdin=cfg([chk("a", s.url("/ok"))]))
        rc_is(r, 0)
        check(r.err == "", "clean run => empty stderr", r)


if __name__ == "__main__":
    sys.exit(main(fixtures_dir=FIX))
