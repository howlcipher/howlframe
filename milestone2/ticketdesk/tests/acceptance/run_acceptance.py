#!/usr/bin/env python3
"""Black-box acceptance suite for ticketdesk (milestone2/ticketdesk).

Usage:  run_acceptance.py LAUNCHER [-v] [--only T-R5] ...   run (all or selected, prefix match)
        run_acceptance.py --list                              list test ids and the requirements covered

Launcher contract: LAUNCHER GRANTS [APP_ARG...]   (REQUIREMENTS.md R1.1)
Python 3 standard library only.  Loopback networking, ephemeral ports, temporary directories.
"""
import hashlib
import json
import os
import re
import signal
import socket
import sys
import threading
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from harness import (test, check, main, Server, REQID_RE)  # noqa: E402

TICKET_KEYS = {"id", "title", "body", "priority", "status", "owner", "created_at", "updated_at"}
TOKEN_KEYS = {"id", "name", "role", "created_at", "revoked"}
LEAK_MARKERS = ["Traceback", "panic", "goroutine", "RUNTIME_ERROR", "HANDLER_PANIC", "CAPABILITY",
                "capability", ".howl", '"phase"', "ip=", "opcode", "OpHttp", "runtime error", "fault"]
GENERIC_401 = "Invalid or missing credentials."
INTERNAL_MSG = "An internal error occurred."
ID_RE = re.compile(r"^[A-Za-z0-9_-]{1,64}$")
TOKID_RE = re.compile(r"^tok_[0-9a-f]{16}$")
SECRET_RE = re.compile(r"^td_[0-9a-f]{64}$")
ACCESS_RE = re.compile(r"^access method=([A-Z]+) path=(\S+) status=([0-9]{3}) duration_ms=([0-9]+) request_id=([0-9a-f]{32})$")
FAULT_RE = re.compile(r"^fault request_id=([0-9a-f]{32}) method=([A-Z]+) path=(\S+) error=(.+)$")


# ------------------------------------------------------------------ helpers
def main_srv(ctx):
    return ctx.cache("main", flags="--access-log")


def err(r, status, code, fields=None):
    """Assert the single error shape (R8.1) and the (status, code) pairing; fields: set of (field, code)."""
    check(r.status == status, "expected HTTP %d, got %d" % (status, r.status), r)
    ct = r.h("content-type") or ""
    check(ct.lower().startswith("application/json"), "Content-Type must be application/json, got %r" % ct, r)
    rid = r.h("x-request-id")
    check(rid and REQID_RE.match(rid), "X-Request-Id must be 32 lowercase hex, got %r" % rid, r)
    j = r.json
    check(isinstance(j, dict) and set(j) == {"error"}, "body must be exactly {\"error\":{...}}", r)
    e = j["error"]
    check(isinstance(e, dict), "error must be an object", r)
    allowed = {"code", "message", "request_id", "fields"}
    check(set(e) <= allowed and {"code", "message", "request_id"} <= set(e), "error keys wrong: %s" % sorted(e), r)
    check(e["code"] == code, "expected code %r, got %r" % (code, e["code"]), r)
    check(isinstance(e["message"], str) and e["message"], "message must be a non-empty string", r)
    check(e["request_id"] == rid, "request_id must equal X-Request-Id header", r)
    if code == "validation_failed":
        fl = e.get("fields")
        check(isinstance(fl, list) and fl, "validation_failed needs a non-empty fields list", r)
        for f in fl:
            check(isinstance(f, dict) and set(f) == {"field", "code", "message"}, "fields entry keys wrong: %r" % (f,), r)
            check(all(isinstance(f[k], str) and f[k] for k in f), "fields entry values must be non-empty strings", r)
        if fields is not None:
            got = {(f["field"], f["code"]) for f in fl}
            check(got == set(fields), "fields mismatch: want %s got %s" % (sorted(fields), sorted(got)), r)
            check(len(fl) == len(got), "one fields entry per field expected", r)
    else:
        check("fields" not in e, "fields only allowed for validation_failed", r)
    return e


def ok(r, status=200):
    check(r.status == status, "expected HTTP %d, got %d" % (status, r.status), r)
    ct = r.h("content-type") or ""
    check(ct.lower().startswith("application/json"), "Content-Type must be application/json, got %r" % ct, r)
    check(REQID_RE.match(r.h("x-request-id") or ""), "X-Request-Id missing/invalid", r)
    check(r.json is not None, "body must be JSON", r)
    return r.json


def is_ticket(t):
    check(isinstance(t, dict) and set(t) == TICKET_KEYS, "ticket keys wrong: %r" % (sorted(t) if isinstance(t, dict) else t))
    check(isinstance(t["id"], str) and ID_RE.match(t["id"]), "ticket id format: %r" % t["id"])
    for k in ("title", "body", "priority", "status", "owner"):
        check(isinstance(t[k], str), "%s must be string" % k)
    check(t["priority"] in ("low", "normal", "high"), "priority enum")
    check(t["status"] in ("open", "in_progress", "done"), "status enum")
    for k in ("created_at", "updated_at"):
        check(type(t[k]) is int and t[k] > 0, "%s must be positive int" % k)
    check(t["updated_at"] >= t["created_at"], "updated_at >= created_at")
    return t


def mk_token(s, role, name="tk"):
    r = s.req("POST", "/v1/tokens", s.admin, {"name": name, "role": role})
    j = ok(r, 201)
    s.secrets.append(j["token"])
    return j["id"], j["token"]


def mk_ticket(s, token, **f):
    body = {"title": "t"}
    body.update(f)
    return is_ticket(ok(s.req("POST", "/v1/tickets", token, body), 201))


def walk(s, token, path="/v1/tickets", **q):
    """Follow cursors; return (pages, items)."""
    pages, items, cursor = [], [], None
    for _ in range(200):
        params = dict(q)
        if cursor:
            params["cursor"] = cursor
        qs = "&".join("%s=%s" % kv for kv in params.items())
        j = ok(s.req("GET", path + ("?" + qs if qs else ""), token))
        check(set(j) == {"items", "next_cursor"}, "list keys wrong: %r" % sorted(j))
        pages.append(j)
        items.extend(j["items"])
        cursor = j["next_cursor"]
        if cursor is None:
            return pages, items
        check(isinstance(cursor, str) and re.match(r"^[A-Za-z0-9_-]{1,64}$", cursor), "bad cursor syntax %r" % cursor)
    raise AssertionError("pagination did not terminate")


def no_internals(text, extra=()):
    for m in LEAK_MARKERS + list(extra):
        if m in ("fault",):
            continue
        check(m not in text, "internal detail leaked to client: %r in %r" % (m, text[:300]))


def stderr_lines(s):
    return [ln for ln in s.stderr().split("\n") if ln != ""]


def wait_for(fn, timeout=3.0):
    t0 = time.time()
    while time.time() - t0 < timeout:
        v = fn()
        if v:
            return v
        time.sleep(0.05)
    return fn()


# =================================================================== R1 startup
@test("T-R1-ready-and-sigterm", ["R1", "R4"], "starts, answers /v1/health, exits within 5s of SIGTERM")
def _(ctx):
    with ctx.fresh() as s:
        r = s.req("GET", "/v1/health")
        check(ok(r) == {"status": "ok"}, "health body", r)
        rc = s.stop(wait=5)
        check(rc is not None, "server did not exit within 5s of SIGTERM")


def startup_fail(ctx, rc_want, **kw):
    res, _s = ctx.try_start(**kw)
    check(not res.hung, "startup failure must exit within 15s (hung)", res)
    check(res.rc == rc_want, "expected exit %d, got %r" % (rc_want, res.rc), res)
    lines = [ln for ln in res.err.split("\n") if ln.strip()]
    check(any(ln.startswith("ticketdesk: ") for ln in lines), "stderr needs a line starting 'ticketdesk: '", res)
    for needle in ('"phase":', "Traceback", "panic:", "goroutine "):
        check(needle not in res.both, "raw runtime/host error leaked (%s)" % needle, res)
    return res


@test("T-R1-missing-admin-token", ["R1"], "no TICKETDESK_ADMIN_TOKEN: exit 2")
def _(ctx):
    startup_fail(ctx, 2, admin=None)


@test("T-R1-bad-admin-token", ["R1"], "empty / <16 chars / space / non-ASCII admin token: exit 2")
def _(ctx):
    for bad in ["", "short-15-chars!", "has a space in it 12345", "unicode-é-token-123456", "tab\there-token-123456"]:
        startup_fail(ctx, 2, admin=bad)


@test("T-R1-admin-token-length-bounds", ["R1"], "16-char and 256-char admin tokens are accepted")
def _(ctx):
    for tok in ["a" * 16, "Z9" * 128]:
        with ctx.fresh(admin=tok) as s:
            check(ok(s.req("GET", "/v1/me", tok))["role"] == "admin", "admin token of length %d must work" % len(tok))


@test("T-R1-missing-data", ["R1"], "no TICKETDESK_DATA: exit 2")
def _(ctx):
    startup_fail(ctx, 2, data=None)


@test("T-R1-bad-port", ["R1"], "TICKETDESK_PORT abc / 0 / 70000 / -1: exit 2")
def _(ctx):
    for bad in ["abc", "0", "70000", "-1", "80x"]:
        startup_fail(ctx, 2, env={"TICKETDESK_PORT": bad}, port=None)


@test("T-R1-corrupt-data", ["R1", "R13"], "garbage in the data file: exit 65, file untouched")
def _(ctx):
    d = ctx.tmp()
    p = os.path.join(d, "ticketdesk.state")
    junk = b"this is not ticketdesk state \x00\xff\xfe {{{"
    with open(p, "wb") as f:
        f.write(junk)
    startup_fail(ctx, 65, data=p, data_dir=d)
    with open(p, "rb") as f:
        check(f.read() == junk, "a failed start must not modify the data file")


@test("T-R1-data-is-directory", ["R1"], "TICKETDESK_DATA is a directory: exit 65")
def _(ctx):
    d = ctx.tmp()
    sub = os.path.join(d, "adir")
    os.mkdir(sub)
    startup_fail(ctx, 65, data=sub, data_dir=d)


@test("T-R1-missing-data-dir", ["R1"], "directory of TICKETDESK_DATA missing: exit 66")
def _(ctx):
    d = ctx.tmp()
    p = os.path.join(d, "nope", "ticketdesk.state")
    startup_fail(ctx, 66, data=p, grants=Server.exact_grants(d), data_dir=d)


@test("T-R1-empty-data-file-ok", ["R1", "R13"], "zero-byte data file is valid empty state")
def _(ctx):
    d = ctx.tmp()
    p = os.path.join(d, "ticketdesk.state")
    open(p, "wb").close()
    with ctx.fresh(data=p, data_dir=d) as s:
        j = ok(s.req("GET", "/v1/tickets", s.admin))
        check(j["items"] == [] and j["next_cursor"] is None, "fresh state must be empty", s.req("GET", "/v1/tickets", s.admin))


@test("T-R1-error-precedence", ["R1"], "env problems (2) win over data (66) problems")
def _(ctx):
    d = ctx.tmp()
    startup_fail(ctx, 2, admin=None, data=os.path.join(d, "nope", "x"), grants=Server.exact_grants(d), data_dir=d)


@test("T-R1-port-in-use", ["R1"], "busy port: nonzero exit within 15s, no hang")
def _(ctx):
    sk = socket.socket()
    sk.bind(("", 0))
    sk.listen(1)
    try:
        port = sk.getsockname()[1]
        res, _s = ctx.try_start(port=port)
        check(not res.hung, "must exit when the port is busy", res)
        check(res.rc not in (0, None), "exit status must be nonzero", res)
    finally:
        sk.close()


# =================================================================== R2 grants
@test("T-R2-exact-grants-workflow", ["R2", "R5"], "full workflow under exactly the documented five grants")
def _(ctx):
    d = ctx.tmp()
    g = "network,database,environment,filesystem:read=%s,filesystem:write=%s" % (d, d)
    with ctx.fresh(data=os.path.join(d, "s.state"), data_dir=d, grants=g) as s:
        t = mk_ticket(s, s.admin, title="grant test")
        mid, mtok = mk_token(s, "member")
        ok(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"status": "done"}))
        check(ok(s.req("GET", "/v1/tickets/" + t["id"], mtok))["status"] == "done", "read back")
        ok(s.req("DELETE", "/v1/tickets/" + t["id"], s.admin))
        check(s.proc.poll() is None, "server must still be running")


def _missing_grant(name):
    @test("T-R2-missing-" + name.replace(":", "-"), ["R2"], "grant %s missing: clean startup failure, exit 77" % name)
    def _(ctx):
        d = ctx.tmp()
        g = Server.exact_grants(d, omit=name)
        res = startup_fail(ctx, 77, data=os.path.join(d, "s.state"), data_dir=d, grants=g)
        check("capability denied" in res.err.lower(), "stderr must contain 'capability denied'", res)


for _g in ["network", "database", "environment", "filesystem:read", "filesystem:write"]:
    _missing_grant(_g)


@test("T-R2-missing-grant-not-listening", ["R2"], "with a missing grant nothing ever listens")
def _(ctx):
    d = ctx.tmp()
    port = None
    res, s = ctx.try_start(data=os.path.join(d, "s.state"), data_dir=d, grants=Server.exact_grants(d, omit="database"))
    check(res.rc == 77, "exit 77 expected", res)
    try:
        s.req("GET", "/v1/health", timeout=2)
        raise AssertionError("something answers on the port after a failed start")
    except (ConnectionError, OSError):
        pass


# =================================================================== R3 conventions
@test("T-R3-health-public", ["R3", "R4"], "GET /v1/health needs no token, ignores garbage Authorization")
def _(ctx):
    s = main_srv(ctx)
    for kw in ({}, {"auth": "garbage"}, {"auth": "Bearer nope"}):
        r = s.req("GET", "/v1/health", **kw)
        check(ok(r) == {"status": "ok"}, "health body", r)


@test("T-R3-unknown-routes", ["R3", "R8"], "unknown paths: 404 not_found for every method, with or without auth")
def _(ctx):
    s = main_srv(ctx)
    paths = ["/", "/v1", "/v1/", "/health", "/v2/tickets", "/v1/ticket", "/v1/tickets/", "/v1/tickets/a/b",
             "/v1/tokens/", "/v1/tokens/a/b", "/v1/health/", "/v1/me/x", "/V1/tickets", "/favicon.ico"]
    for p in paths:
        for m in ("GET", "POST", "DELETE", "PATCH", "PUT"):
            for auth in (None, s.admin):
                r = s.req(m, p, auth, body={"x": 1} if m in ("POST", "PUT", "PATCH") else None)
                err(r, 404, "not_found")


@test("T-R3-method-not-allowed", ["R3", "R8"], "known path, wrong method: 405 + exact Allow, before authentication")
def _(ctx):
    s = main_srv(ctx)
    table = {
        "/v1/health": ("GET", ["POST", "PUT", "DELETE", "PATCH", "OPTIONS"]),
        "/v1/me": ("GET", ["POST", "PUT", "DELETE", "PATCH", "OPTIONS"]),
        "/v1/tickets": ("GET, POST", ["PUT", "DELETE", "PATCH", "OPTIONS"]),
        "/v1/tickets/abc": ("DELETE, GET, PATCH", ["POST", "PUT", "OPTIONS"]),
        "/v1/tokens": ("GET, POST", ["PUT", "DELETE", "PATCH", "OPTIONS"]),
        "/v1/tokens/abc": ("DELETE", ["GET", "POST", "PUT", "PATCH", "OPTIONS"]),
    }
    for path, (allow, methods) in table.items():
        for m in methods:
            for auth in (None, s.admin):
                r = s.req(m, path, auth, body={} if m in ("POST", "PUT", "PATCH") else None)
                err(r, 405, "method_not_allowed")
                check(r.h("allow") == allow, "Allow for %s: want %r got %r" % (path, allow, r.h("allow")), r)


@test("T-R3-head-not-allowed", ["R3"], "HEAD on every route: 405 with Allow")
def _(ctx):
    s = main_srv(ctx)
    for path, allow in [("/v1/health", "GET"), ("/v1/tickets", "GET, POST"), ("/v1/tickets/x", "DELETE, GET, PATCH"),
                        ("/v1/tokens", "GET, POST"), ("/v1/tokens/x", "DELETE"), ("/v1/me", "GET")]:
        r = s.req("HEAD", path, s.admin)
        check(r.status == 405 and r.h("allow") == allow, "HEAD %s -> %s Allow=%r" % (path, r.status, r.h("allow")), r)
        check(REQID_RE.match(r.h("x-request-id") or ""), "X-Request-Id on HEAD response", r)


@test("T-R3-common-headers", ["R3", "R8"], "Content-Type + X-Request-Id on every response class")
def _(ctx):
    s = main_srv(ctx)
    _, vtok = mk_token(s, "viewer")
    rs = [s.req("GET", "/v1/health"), s.req("GET", "/v1/me", s.admin), s.req("POST", "/v1/tickets", s.admin, {"title": "h"}),
          s.req("POST", "/v1/tickets", s.admin, {}), s.req("GET", "/v1/tickets"), s.req("POST", "/v1/tickets", vtok, {"title": "x"}),
          s.req("GET", "/v1/nope"), s.req("PUT", "/v1/tickets"), s.req("GET", "/v1/tickets/zzz", s.admin),
          s.req("POST", "/v1/tickets", s.admin, "{", ), s.req("POST", "/v1/tickets", s.admin, {"title": "x"}, ctype="text/plain")]
    codes = [200, 200, 201, 400, 401, 403, 404, 405, 404, 400, 415]
    for r, c in zip(rs, codes):
        check(r.status == c, "status %d expected, got %d" % (c, r.status), r)
        check((r.h("content-type") or "").lower().startswith("application/json"), "Content-Type JSON", r)
        check(REQID_RE.match(r.h("x-request-id") or ""), "X-Request-Id", r)
        check(r.json is not None, "JSON body", r)


@test("T-R3-request-id-unique", ["R3"], "X-Request-Id differs for each of 60 requests; error request_id matches it")
def _(ctx):
    s = main_srv(ctx)
    ids = set()
    for i in range(60):
        r = s.req("GET", "/v1/health") if i % 3 == 0 else (s.req("GET", "/v1/tickets") if i % 3 == 1 else s.req("GET", "/v1/nope"))
        rid = r.h("x-request-id")
        check(REQID_RE.match(rid or ""), "request id format", r)
        check(rid not in ids, "duplicate X-Request-Id %s" % rid, r)
        ids.add(rid)
        if r.status >= 400:
            check(r.json["error"]["request_id"] == rid, "body request_id equals header", r)


@test("T-R3-request-id-not-echoed", ["R3"], "client-supplied X-Request-Id is ignored")
def _(ctx):
    s = main_srv(ctx)
    r = s.req("GET", "/v1/health", headers=[("X-Request-Id", "evil-client-id")])
    check(r.h("x-request-id") != "evil-client-id" and REQID_RE.match(r.h("x-request-id") or ""), "client id must not be echoed", r)
    r = s.req("GET", "/v1/nope", headers=[("X-Request-Id", "0" * 32)])
    check(r.h("x-request-id") != "0" * 32, "even a well-formed client id must be replaced", r)


@test("T-R3-location-headers", ["R3", "R5", "R11"], "201 responses carry Location")
def _(ctx):
    s = main_srv(ctx)
    r = s.req("POST", "/v1/tickets", s.admin, {"title": "loc"})
    check(r.h("location") == "/v1/tickets/" + ok(r, 201)["id"], "ticket Location", r)
    r = s.req("POST", "/v1/tokens", s.admin, {"name": "loc", "role": "viewer"})
    check(r.h("location") == "/v1/tokens/" + ok(r, 201)["id"], "token Location", r)


@test("T-R3-check-order", ["R3", "R8", "R10"], "order: route, method, auth, role, existence, ownership, media type, size, JSON, fields")
def _(ctx):
    s = main_srv(ctx)
    _, vtok = mk_token(s, "viewer")
    _, mtok = mk_token(s, "member")
    t = mk_ticket(s, s.admin)
    err(s.req("GET", "/v1/tickets/nosuch"), 401, "unauthorized")                                  # auth before existence
    err(s.req("PATCH", "/v1/tickets/nosuch", vtok, {"title": "x"}), 403, "forbidden")             # role before existence
    err(s.req("PATCH", "/v1/tickets/nosuch", mtok, {"title": "x"}), 404, "not_found")             # existence
    err(s.req("PATCH", "/v1/tickets/nosuch", mtok, "{", ctype="text/plain"), 404, "not_found")    # existence before media type
    err(s.req("PATCH", "/v1/tickets/" + t["id"], mtok, {"title": "x"}), 403, "forbidden")         # ownership
    err(s.req("PATCH", "/v1/tickets/" + t["id"], mtok, "{", ctype="text/plain"), 403, "forbidden")  # ownership before media type
    err(s.req("POST", "/v1/tickets", vtok, "{", ctype="text/plain"), 403, "forbidden")            # role before media type
    err(s.req("POST", "/v1/tickets", s.admin, "{", ctype="text/plain"), 415, "unsupported_media_type")  # media type before JSON
    err(s.req("POST", "/v1/tickets", None, {"title": "x"}), 401, "unauthorized")
    err(s.req("PUT", "/v1/tickets"), 405, "method_not_allowed")                                   # method before auth


@test("T-R3-media-type", ["R3", "R7", "R8"], "Content-Type negotiation: accepted and rejected media types")
def _(ctx):
    s = main_srv(ctx)
    for ct in ["application/json", "Application/JSON", "application/json; charset=utf-8", "application/json;charset=UTF-8"]:
        ok(s.req("POST", "/v1/tickets", s.admin, {"title": "ct"}, ctype=ct), 201)
    for ct in [None, "text/plain", "application/x-www-form-urlencoded", "application/jsonx", "text/json", "multipart/form-data; boundary=x", ""]:
        err(s.req("POST", "/v1/tickets", s.admin, {"title": "ct"}, ctype=ct), 415, "unsupported_media_type")
    err(s.req("POST", "/v1/tokens", s.admin, {"name": "x", "role": "viewer"}, ctype="text/plain"), 415, "unsupported_media_type")
    t = mk_ticket(s, s.admin)
    err(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"title": "y"}, ctype="text/plain"), 415, "unsupported_media_type")


@test("T-R3-ignored-inputs", ["R3"], "unknown query params and GET/DELETE bodies are ignored")
def _(ctx):
    s = main_srv(ctx)
    ok(s.req("GET", "/v1/health?x=1&y=2"))
    ok(s.req("GET", "/v1/tickets?zzz=1&limit=2", s.admin))
    r = s.req("GET", "/v1/tickets", s.admin, body="{not json", ctype="text/plain")
    ok(r)
    t = mk_ticket(s, s.admin)
    ok(s.req("DELETE", "/v1/tickets/" + t["id"] + "?force=1", s.admin, body="garbage", ctype="text/plain"))


# =================================================================== R4 identity
@test("T-R4-me", ["R4", "R9"], "/v1/me for bootstrap, member, viewer, admin tokens")
def _(ctx):
    s = main_srv(ctx)
    check(ok(s.req("GET", "/v1/me", s.admin)) == {"id": "bootstrap", "name": "bootstrap", "role": "admin"}, "bootstrap identity")
    for role in ("admin", "member", "viewer"):
        tid, tok = mk_token(s, role, name="n-" + role)
        check(ok(s.req("GET", "/v1/me", tok)) == {"id": tid, "name": "n-" + role, "role": role}, "me for " + role)


# =================================================================== R5 tickets
@test("T-R5-crud-workflow", ["R5", "R3"], "create, read, list, patch, delete, read-404")
def _(ctx):
    with ctx.fresh() as s:
        j = ok(s.req("POST", "/v1/tickets", s.admin, {"title": "Printer on fire", "body": "smoke", "priority": "high"}), 201)
        t = is_ticket(j)
        check((t["title"], t["body"], t["priority"], t["status"], t["owner"]) == ("Printer on fire", "smoke", "high", "open", "bootstrap"), "create values")
        check(is_ticket(ok(s.req("GET", "/v1/tickets/" + t["id"], s.admin))) == t, "GET equals created")
        lst = ok(s.req("GET", "/v1/tickets", s.admin))
        check(lst == {"items": [t], "next_cursor": None}, "list shape/content")
        p = is_ticket(ok(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"status": "in_progress", "title": "Printer cooled"})))
        check((p["status"], p["title"], p["body"], p["priority"], p["id"], p["owner"], p["created_at"]) ==
              ("in_progress", "Printer cooled", "smoke", "high", t["id"], "bootstrap", t["created_at"]), "patched values")
        d = ok(s.req("DELETE", "/v1/tickets/" + t["id"], s.admin))
        check(d == {"id": t["id"], "deleted": True}, "delete body")
        for m in ("GET", "DELETE"):
            err(s.req(m, "/v1/tickets/" + t["id"], s.admin), 404, "not_found")
        err(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"title": "x"}), 404, "not_found")
        check(ok(s.req("GET", "/v1/tickets", s.admin))["items"] == [], "list empty after delete")


@test("T-R5-create-defaults", ["R5"], "defaults: body '', priority normal, status open; exact key set")
def _(ctx):
    s = main_srv(ctx)
    t = mk_ticket(s, s.admin, title="just a title")
    check((t["body"], t["priority"], t["status"]) == ("", "normal", "open"), "defaults")
    check(t["owner"] == "bootstrap", "owner of bootstrap-created ticket")


@test("T-R5-patch-partial", ["R5"], "PATCH changes only the given fields, keeps id/owner/created_at, updated_at non-decreasing")
def _(ctx):
    s = main_srv(ctx)
    t = mk_ticket(s, s.admin, title="orig", body="b", priority="low")
    for patch in ({"title": "T2"}, {"body": ""}, {"priority": "high"}, {"status": "done"}, {"title": "T3", "status": "open"}):
        before = ok(s.req("GET", "/v1/tickets/" + t["id"], s.admin))
        after = is_ticket(ok(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, patch)))
        want = dict(before)
        want.update(patch)
        for k in ("updated_at",):
            check(after[k] >= before[k], "updated_at must not decrease")
            want[k] = after[k]
        check(after == want, "only %s may change: before=%r after=%r" % (sorted(patch), before, after))


@test("T-R5-ids", ["R5"], "ids unique, well-formed and never reused after delete")
def _(ctx):
    with ctx.fresh() as s:
        seen = []
        for i in range(12):
            t = mk_ticket(s, s.admin, title="n%d" % i)
            check(t["id"] not in seen, "duplicate id")
            seen.append(t["id"])
            if i % 3 == 0:
                ok(s.req("DELETE", "/v1/tickets/" + t["id"], s.admin))
        t = mk_ticket(s, s.admin)
        check(t["id"] not in seen, "id of a deleted/existing ticket was reused")


@test("T-R5-unicode-roundtrip", ["R5", "R7"], "unicode, quotes, newlines and escapes round-trip exactly")
def _(ctx):
    s = main_srv(ctx)
    title = 'Café 日本語 \U0001F525 "quoted" \\ back\\slash'
    body = 'line1\nline2\r\n\ttab </script>   \u0000nul-less'.replace("\u0000", "")
    t = mk_ticket(s, s.admin, title=title, body=body)
    check((t["title"], t["body"]) == (title, body), "create echo")
    g = ok(s.req("GET", "/v1/tickets/" + t["id"], s.admin))
    check((g["title"], g["body"]) == (title, body), "read back")
    raw = s.req("POST", "/v1/tickets", s.admin, b'{"title":"\\u00e9\\ud83d\\udd25 \\"x\\"","body":"a\\nb"}')
    j = ok(raw, 201)
    check((j["title"], j["body"]) == ('é\U0001F525 "x"', "a\nb"), "JSON escapes decoded")


@test("T-R5-unknown-id-variants", ["R5", "R8"], "odd ids are plain 404s")
def _(ctx):
    s = main_srv(ctx)
    for i in ["zzz", "0", "t999999999", "a%20b", "%2e%2e", "x" * 200, "t1;x", "caf%C3%A9"]:
        for m, b in (("GET", None), ("DELETE", None), ("PATCH", {"title": "x"})):
            err(s.req(m, "/v1/tickets/" + i, s.admin, b), 404, "not_found")


# =================================================================== R6 listing
@test("T-R6-pagination-walk", ["R6"], "limit/cursor walk returns everything once, in creation order; exact end has null cursor")
def _(ctx):
    with ctx.fresh() as s:
        made = [mk_ticket(s, s.admin, title="p%02d" % i)["id"] for i in range(7)]
        pages, items = walk(s, s.admin, limit=3)
        check([len(p["items"]) for p in pages] == [3, 3, 1], "page sizes %r" % [len(p["items"]) for p in pages])
        check([t["id"] for t in items] == made, "order/completeness")
        pages, items = walk(s, s.admin, limit=7)
        check(len(pages) == 1 and pages[0]["next_cursor"] is None, "page ending exactly at the end has next_cursor null")
        pages, items = walk(s, s.admin, limit=1)
        check(len(pages) == 7 and [t["id"] for t in items] == made, "limit=1 walk")
        j = ok(s.req("GET", "/v1/tickets?limit=3", s.admin))
        check(j["next_cursor"] is not None and len(j["items"]) == 3, "first page has a cursor")
        j2 = ok(s.req("GET", "/v1/tickets?limit=3&cursor=" + j["next_cursor"], s.admin))
        j3 = ok(s.req("GET", "/v1/tickets?limit=3&cursor=" + j["next_cursor"], s.admin))
        check(j2 == j3, "same cursor, same page")


@test("T-R6-default-limit", ["R6"], "default limit 20, max 100 accepted")
def _(ctx):
    with ctx.fresh() as s:
        for i in range(25):
            mk_ticket(s, s.admin, title="d%d" % i)
        j = ok(s.req("GET", "/v1/tickets", s.admin))
        check(len(j["items"]) == 20 and j["next_cursor"], "default page = 20 with cursor")
        pages, items = walk(s, s.admin)
        check([len(p["items"]) for p in pages] == [20, 5], "20 + 5")
        j = ok(s.req("GET", "/v1/tickets?limit=100", s.admin))
        check(len(j["items"]) == 25 and j["next_cursor"] is None, "limit=100 returns all 25")
        check(len(ok(s.req("GET", "/v1/tickets?limit=1", s.admin))["items"]) == 1, "limit=1")


@test("T-R6-status-filter", ["R6"], "status filter applies before paging")
def _(ctx):
    with ctx.fresh() as s:
        ids = {"open": [], "in_progress": [], "done": []}
        for i in range(9):
            t = mk_ticket(s, s.admin, title="f%d" % i)
            st = ["open", "in_progress", "done"][i % 3]
            if st != "open":
                ok(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"status": st}))
            ids[st].append(t["id"])
        for st, want in ids.items():
            _, items = walk(s, s.admin, status=st)
            check([t["id"] for t in items] == want and all(t["status"] == st for t in items), "filter " + st)
            pages, items = walk(s, s.admin, status=st, limit=2)
            check([len(p["items"]) for p in pages] == [2, 1] and [t["id"] for t in items] == want, "filter+paging " + st)
        j = ok(s.req("GET", "/v1/tickets?status=done&limit=3", s.admin))
        check(j["next_cursor"] is None and len(j["items"]) == 3, "exact end with filter has null cursor")


@test("T-R6-invalid-query", ["R6", "R7", "R8"], "invalid limit/status/cursor: 400 validation_failed invalid_value")
def _(ctx):
    s = main_srv(ctx)
    for q, f in [("limit=0", "limit"), ("limit=-1", "limit"), ("limit=101", "limit"), ("limit=abc", "limit"),
                 ("limit=1.5", "limit"), ("limit=1e1", "limit"), ("limit=%2B5", "limit"), ("limit=99999999999999999999", "limit"),
                 ("status=bogus", "status"), ("status=OPEN", "status"), ("status=in-progress", "status"),
                 ("cursor=!!!", "cursor"), ("cursor=a%20b", "cursor"), ("cursor=" + "x" * 65, "cursor")]:
        err(s.req("GET", "/v1/tickets?" + q, s.admin), 400, "validation_failed", {(f, "invalid_value")})
    err(s.req("GET", "/v1/tickets?limit=0&status=bogus&cursor=%21", s.admin), 400, "validation_failed",
        {("limit", "invalid_value"), ("status", "invalid_value"), ("cursor", "invalid_value")})


@test("T-R6-empty-query-values", ["R6"], "empty ?limit= ?status= ?cursor= behave as absent")
def _(ctx):
    with ctx.fresh() as s:
        mk_ticket(s, s.admin)
        j = ok(s.req("GET", "/v1/tickets?limit=&status=&cursor=", s.admin))
        check(len(j["items"]) == 1, "empty values ignored")


@test("T-R6-delete-during-walk", ["R6"], "deleting tickets between pages does not break the walk")
def _(ctx):
    with ctx.fresh() as s:
        ids = [mk_ticket(s, s.admin, title="w%d" % i)["id"] for i in range(6)]
        j = ok(s.req("GET", "/v1/tickets?limit=2", s.admin))
        check([t["id"] for t in j["items"]] == ids[:2], "first page")
        ok(s.req("DELETE", "/v1/tickets/" + ids[1], s.admin))
        ok(s.req("DELETE", "/v1/tickets/" + ids[2], s.admin))
        j2 = ok(s.req("GET", "/v1/tickets?limit=10&cursor=" + j["next_cursor"], s.admin))
        check([t["id"] for t in j2["items"]] == ids[3:] and j2["next_cursor"] is None, "continues after the cursor position")


@test("T-R6-order-stable-under-edit", ["R6", "R5"], "editing an old ticket does not move it in the list")
def _(ctx):
    with ctx.fresh() as s:
        ids = [mk_ticket(s, s.admin, title="o%d" % i)["id"] for i in range(5)]
        ok(s.req("PATCH", "/v1/tickets/" + ids[0], s.admin, {"status": "done", "title": "edited"}))
        _, items = walk(s, s.admin, limit=2)
        check([t["id"] for t in items] == ids, "creation order preserved")


@test("T-R6-visible-to-all-roles", ["R6", "R10"], "every role sees all tickets")
def _(ctx):
    with ctx.fresh() as s:
        _, m1 = mk_token(s, "member")
        _, m2 = mk_token(s, "member")
        _, v = mk_token(s, "viewer")
        a = mk_ticket(s, m1)["id"]
        b = mk_ticket(s, m2)["id"]
        for tok in (m1, m2, v, s.admin):
            _, items = walk(s, tok)
            check([t["id"] for t in items] == [a, b], "all tickets visible")


# =================================================================== R7 validation
@test("T-R7-malformed-json", ["R7", "R8"], "syntactically broken bodies: 400 invalid_json (POST and PATCH)")
def _(ctx):
    s = main_srv(ctx)
    t = mk_ticket(s, s.admin)
    bodies = ["", "   ", "\n", "{", '{"title":', "not json", '{"title":"x"} x', '{"title":"x",}', "{'title':'x'}",
              b"\xff\xfe\x00{", '{"title":"\xff"}'.encode("latin-1"), "NaN", '{"title":"x"}{"title":"y"}', "}{", "nul\u0000l"]
    for b in bodies:
        err(s.req("POST", "/v1/tickets", s.admin, b), 400, "invalid_json")
        err(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, b), 400, "invalid_json")
    err(s.req("POST", "/v1/tokens", s.admin, "{"), 400, "invalid_json")
    check(ok(s.req("GET", "/v1/tickets/" + t["id"], s.admin)) == t, "ticket untouched by rejected patches")


@test("T-R7-non-object-json", ["R7", "R8"], "valid JSON that is not an object: validation_failed on $")
def _(ctx):
    s = main_srv(ctx)
    t = mk_ticket(s, s.admin)
    for b in ["[]", '"x"', "1", "null", "true", "[{\"title\":\"x\"}]", "1.5"]:
        err(s.req("POST", "/v1/tickets", s.admin, b), 400, "validation_failed", {("$", "invalid_type")})
        err(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, b), 400, "validation_failed", {("$", "invalid_type")})
        err(s.req("POST", "/v1/tokens", s.admin, b), 400, "validation_failed", {("$", "invalid_type")})


@test("T-R7-title-rules", ["R7"], "title required, string, 1..120")
def _(ctx):
    s = main_srv(ctx)
    P = lambda b: s.req("POST", "/v1/tickets", s.admin, b)  # noqa: E731
    err(P({}), 400, "validation_failed", {("title", "required")})
    err(P({"body": "x"}), 400, "validation_failed", {("title", "required")})
    err(P({"title": ""}), 400, "validation_failed", {("title", "too_short")})
    err(P({"title": "x" * 121}), 400, "validation_failed", {("title", "too_long")})
    for v in (5, 1.5, True, False, None, [], ["a"], {}, {"a": 1}):
        err(P({"title": v}), 400, "validation_failed", {("title", "invalid_type")})
    for v in ("x", " ", "x" * 120, "0"):
        check(ok(P({"title": v}), 201)["title"] == v, "valid title %r" % v[:10])


@test("T-R7-length-code-points", ["R7"], "lengths count code points, not bytes")
def _(ctx):
    s = main_srv(ctx)
    P = lambda b: s.req("POST", "/v1/tickets", s.admin, b)  # noqa: E731
    for ch in ("é", "日", "\U0001F525"):
        ok(P({"title": ch * 120}), 201)
        err(P({"title": ch * 121}), 400, "validation_failed", {("title", "too_long")})
    ok(P({"title": "x", "body": "\U0001F525" * 4000}), 201)
    err(P({"title": "x", "body": "\U0001F525" * 4001}), 400, "validation_failed", {("body", "too_long")})


@test("T-R7-body-rules", ["R7"], "body optional string 0..4000")
def _(ctx):
    s = main_srv(ctx)
    P = lambda b: s.req("POST", "/v1/tickets", s.admin, b)  # noqa: E731
    for v in ("", "x", "y" * 4000):
        check(ok(P({"title": "b", "body": v}), 201)["body"] == v, "valid body")
    err(P({"title": "b", "body": "y" * 4001}), 400, "validation_failed", {("body", "too_long")})
    for v in (None, 5, True, [], {}):
        err(P({"title": "b", "body": v}), 400, "validation_failed", {("body", "invalid_type")})


@test("T-R7-priority-rules", ["R7"], "priority enum low|normal|high, strict type")
def _(ctx):
    s = main_srv(ctx)
    P = lambda b: s.req("POST", "/v1/tickets", s.admin, b)  # noqa: E731
    for v in ("low", "normal", "high"):
        check(ok(P({"title": "p", "priority": v}), 201)["priority"] == v, "valid priority")
    for v in ("HIGH", "High", "", "urgent", " high", "high "):
        err(P({"title": "p", "priority": v}), 400, "validation_failed", {("priority", "invalid_value")})
    for v in (1, True, None, [], {}):
        err(P({"title": "p", "priority": v}), 400, "validation_failed", {("priority", "invalid_type")})


@test("T-R7-status-rules-patch", ["R7"], "status enum on PATCH, strict type")
def _(ctx):
    s = main_srv(ctx)
    t = mk_ticket(s, s.admin)
    Pa = lambda b: s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, b)  # noqa: E731
    for v in ("open", "in_progress", "done", "open"):
        check(ok(Pa({"status": v}))["status"] == v, "valid status")
    for v in ("closed", "DONE", "", "in progress", "in-progress"):
        err(Pa({"status": v}), 400, "validation_failed", {("status", "invalid_value")})
    for v in (0, False, None, [], {}):
        err(Pa({"status": v}), 400, "validation_failed", {("status", "invalid_type")})


@test("T-R7-unknown-fields", ["R7", "R8"], "unknown fields are rejected with unknown_field per key")
def _(ctx):
    s = main_srv(ctx)
    t = mk_ticket(s, s.admin)
    err(s.req("POST", "/v1/tickets", s.admin, {"title": "u", "extra": 1}), 400, "validation_failed", {("extra", "unknown_field")})
    for k in ("status", "id", "owner", "created_at", "updated_at", "Title", "TITLE"):
        err(s.req("POST", "/v1/tickets", s.admin, {"title": "u", k: "x"}), 400, "validation_failed", {(k, "unknown_field")})
    for k in ("id", "owner", "created_at", "updated_at", "extra"):
        err(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"title": "ok", k: 1}), 400, "validation_failed", {(k, "unknown_field")})
    err(s.req("POST", "/v1/tickets", s.admin, {"title": "u", "a": 1, "b": 2}), 400, "validation_failed",
        {("a", "unknown_field"), ("b", "unknown_field")})
    err(s.req("POST", "/v1/tokens", s.admin, {"name": "n", "role": "viewer", "token": "x"}), 400, "validation_failed", {("token", "unknown_field")})


@test("T-R7-multiple-violations", ["R7", "R8"], "all violated fields reported together")
def _(ctx):
    s = main_srv(ctx)
    err(s.req("POST", "/v1/tickets", s.admin, {"title": "", "priority": "urgent", "body": 5, "zzz": 1}), 400, "validation_failed",
        {("title", "too_short"), ("priority", "invalid_value"), ("body", "invalid_type"), ("zzz", "unknown_field")})
    t = mk_ticket(s, s.admin)
    err(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"title": "x" * 200, "status": "no", "priority": 3}), 400, "validation_failed",
        {("title", "too_long"), ("status", "invalid_value"), ("priority", "invalid_type")})


@test("T-R7-patch-empty-and-atomic", ["R7", "R5"], "{} is rejected; invalid patch changes nothing")
def _(ctx):
    s = main_srv(ctx)
    t = mk_ticket(s, s.admin, title="keep", priority="low")
    err(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {}), 400, "validation_failed", {("$", "required")})
    err(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"title": "changed", "status": "bogus"}), 400, "validation_failed",
        {("status", "invalid_value")})
    check(ok(s.req("GET", "/v1/tickets/" + t["id"], s.admin)) == t, "failed PATCH must not partially apply")


@test("T-R7-token-fields", ["R7", "R11"], "token name/role validation")
def _(ctx):
    s = main_srv(ctx)
    P = lambda b: s.req("POST", "/v1/tokens", s.admin, b)  # noqa: E731
    err(P({}), 400, "validation_failed", {("name", "required"), ("role", "required")})
    err(P({"role": "viewer"}), 400, "validation_failed", {("name", "required")})
    err(P({"name": "n"}), 400, "validation_failed", {("role", "required")})
    err(P({"name": "", "role": "viewer"}), 400, "validation_failed", {("name", "too_short")})
    err(P({"name": "n" * 65, "role": "viewer"}), 400, "validation_failed", {("name", "too_long")})
    err(P({"name": 5, "role": "viewer"}), 400, "validation_failed", {("name", "invalid_type")})
    err(P({"name": "n", "role": "root"}), 400, "validation_failed", {("role", "invalid_value")})
    err(P({"name": "n", "role": "ADMIN"}), 400, "validation_failed", {("role", "invalid_value")})
    err(P({"name": "n", "role": None}), 400, "validation_failed", {("role", "invalid_type")})
    ok(P({"name": "n" * 64, "role": "viewer"}), 201)
    ok(P({"name": "x", "role": "member"}), 201)


# =================================================================== R8 error model
@test("T-R8-code-table", ["R8"], "each documented error code appears with exactly its documented status")
def _(ctx):
    s = main_srv(ctx)
    _, vtok = mk_token(s, "viewer")
    t = mk_ticket(s, s.admin)
    cases = [
        (s.req("POST", "/v1/tickets", s.admin, "{"), 400, "invalid_json"),
        (s.req("POST", "/v1/tickets", s.admin, {}), 400, "validation_failed"),
        (s.req("GET", "/v1/tickets"), 401, "unauthorized"),
        (s.req("DELETE", "/v1/tickets/" + t["id"], vtok), 403, "forbidden"),
        (s.req("GET", "/v1/tickets/none", s.admin), 404, "not_found"),
        (s.req("PUT", "/v1/tickets", s.admin), 405, "method_not_allowed"),
        (s.req("DELETE", "/v1/tokens/bootstrap", s.admin), 409, "conflict"),
        (s.req("POST", "/v1/tickets", s.admin, {"title": "x"}, ctype="text/plain"), 415, "unsupported_media_type"),
        (s.req("POST", "/v1/tickets", s.admin, b" " * 1048577), 413, "payload_too_large"),
    ]
    for r, st, code in cases:
        err(r, st, code)


@test("T-R8-401-message-identical", ["R8", "R9"], "every 401 has the same fixed message")
def _(ctx):
    s = main_srv(ctx)
    tid, tok = mk_token(s, "member")
    ok(s.req("DELETE", "/v1/tokens/" + tid, s.admin))
    msgs = set()
    for kw in ({}, {"auth": "Basic abc"}, {"auth": "Bearer"}, {"auth": "Bearer nope"}, {"token": tok}, {"auth": "Bearer " + s.admin.upper()}):
        e = err(s.req("GET", "/v1/tickets", **kw), 401, "unauthorized")
        msgs.add(e["message"])
    check(msgs == {GENERIC_401}, "401 messages: %r" % msgs)


@test("T-R8-no-secret-in-errors", ["R8", "R12"], "error bodies never echo tokens or Authorization values")
def _(ctx):
    s = main_srv(ctx)
    _, tok = mk_token(s, "viewer")
    rs = [s.req("POST", "/v1/tickets", tok, {"title": "x"}), s.req("GET", "/v1/tickets", auth="Bearer " + tok + "x"),
          s.req("GET", "/v1/tickets", auth="Bearer a b " + tok), s.req("PUT", "/v1/tickets", tok),
          s.req("POST", "/v1/tokens", s.admin, {"name": tok, "role": "bad"}), s.req("GET", "/v1/tickets?cursor=" + tok + "!", s.admin)]
    for r in rs:
        check(tok not in r.text and s.admin not in r.text, "secret echoed in error body", r)
        check("Bearer" not in r.text, "Authorization value echoed", r)
        no_internals(r.text)


# =================================================================== R9 authentication
@test("T-R9-garbled-credentials", ["R9", "R8"], "no/garbled/unknown credentials: 401 + WWW-Authenticate")
def _(ctx):
    s = main_srv(ctx)
    for auth in [None, "", "Basic dXNlcjpwYXNz", "Token " + s.admin, "Bearer", "Bearer ", "Bearer a b", "Bearer  " + s.admin,
                 "Bearer\t" + s.admin, s.admin, "Bearer nope", "Bearer " + s.admin + "x", "Bearer " + s.admin[:-1],
                 "Bearer " + s.admin.upper(), "Bearer td_" + "0" * 64, "Bearer " + "A" * 5000, "Bearer é", "Negotiate xyz"]:
        for path in ("/v1/tickets", "/v1/me", "/v1/tokens"):
            r = s.req("GET", path, auth=auth) if auth is not None else s.req("GET", path)
            err(r, 401, "unauthorized")
            check(r.h("www-authenticate") == 'Bearer realm="ticketdesk"', "WWW-Authenticate: %r" % r.h("www-authenticate"), r)
    r = s.req("POST", "/v1/tickets", None, {"title": "x"})
    err(r, 401, "unauthorized")


@test("T-R9-scheme-case", ["R9"], "Bearer scheme is case-insensitive")
def _(ctx):
    s = main_srv(ctx)
    for scheme in ("Bearer", "bearer", "BEARER", "bEaReR"):
        ok(s.req("GET", "/v1/me", auth=scheme + " " + s.admin))


@test("T-R9-repeated-header", ["R9"], "with repeated Authorization headers the first value counts")
def _(ctx):
    s = main_srv(ctx)
    ok(s.req("GET", "/v1/me", auth=["Bearer " + s.admin, "Bearer wrong"]))
    err(s.req("GET", "/v1/me", auth=["Bearer wrong", "Bearer " + s.admin]), 401, "unauthorized")


@test("T-R9-unauth-writes-do-nothing", ["R9"], "unauthenticated mutations do not mutate")
def _(ctx):
    with ctx.fresh() as s:
        t = mk_ticket(s, s.admin)
        err(s.req("POST", "/v1/tickets", None, {"title": "no"}), 401, "unauthorized")
        err(s.req("PATCH", "/v1/tickets/" + t["id"], None, {"title": "no"}), 401, "unauthorized")
        err(s.req("DELETE", "/v1/tickets/" + t["id"], None), 401, "unauthorized")
        err(s.req("POST", "/v1/tokens", None, {"name": "n", "role": "admin"}), 401, "unauthorized")
        items = ok(s.req("GET", "/v1/tickets", s.admin))["items"]
        check(items == [t], "state unchanged")
        check(ok(s.req("GET", "/v1/tokens", s.admin))["items"] == [], "no token created")


# =================================================================== R10 authorization
@test("T-R10-matrix", ["R10"], "role x operation matrix (R10 table)")
def _(ctx):
    with ctx.fresh() as s:
        toks = {"admin": mk_token(s, "admin")[1], "member": None, "viewer": mk_token(s, "viewer")[1]}
        mid, toks["member"] = mk_token(s, "member")
        for role in ("admin", "member", "viewer"):
            tok = toks[role]
            own = mk_ticket(s, toks["member"], title="m-own") if role == "member" else mk_ticket(s, s.admin, title="a-own")
            other = mk_ticket(s, s.admin, title="other")
            # reads
            ok(s.req("GET", "/v1/me", tok))
            ok(s.req("GET", "/v1/tickets", tok))
            ok(s.req("GET", "/v1/tickets/" + other["id"], tok))
            # create
            r = s.req("POST", "/v1/tickets", tok, {"title": "c-" + role})
            if role == "viewer":
                err(r, 403, "forbidden")
            else:
                ok(r, 201)
            # patch own / other
            ownr = s.req("PATCH", "/v1/tickets/" + own["id"], tok, {"priority": "high"})
            othr = s.req("PATCH", "/v1/tickets/" + other["id"], tok, {"priority": "high"})
            if role == "admin":
                ok(ownr); ok(othr)
            elif role == "member":
                ok(ownr); err(othr, 403, "forbidden")
            else:
                err(ownr, 403, "forbidden"); err(othr, 403, "forbidden")
            # delete
            if role == "admin":
                ok(s.req("DELETE", "/v1/tickets/" + other["id"], tok))
            else:
                err(s.req("DELETE", "/v1/tickets/" + own["id"], tok), 403, "forbidden")
                err(s.req("DELETE", "/v1/tickets/" + other["id"], tok), 403, "forbidden")
            # tokens
            t1 = s.req("GET", "/v1/tokens", tok)
            t2 = s.req("POST", "/v1/tokens", tok, {"name": "z-" + role, "role": "viewer"})
            t3 = s.req("DELETE", "/v1/tokens/" + mid, tok) if role != "member" else s.req("DELETE", "/v1/tokens/tok_0000000000000000", tok)
            if role == "admin":
                ok(t1); ok(t2, 201)
                ok(t3)
                _, toks["member"] = mk_token(s, "member")  # re-create since member token revoked above
            else:
                err(t1, 403, "forbidden"); err(t2, 403, "forbidden"); err(t3, 403, "forbidden")


@test("T-R10-ownership", ["R10"], "member edits only own tickets; admin edits any without changing owner")
def _(ctx):
    with ctx.fresh() as s:
        aid, a = mk_token(s, "member", "alice")
        bid, b = mk_token(s, "member", "bob")
        t = mk_ticket(s, a, title="alice's")
        check(t["owner"] == aid, "owner is the creator's token id")
        err(s.req("PATCH", "/v1/tickets/" + t["id"], b, {"title": "bob was here"}), 403, "forbidden")
        err(s.req("DELETE", "/v1/tickets/" + t["id"], b), 403, "forbidden")
        check(ok(s.req("PATCH", "/v1/tickets/" + t["id"], a, {"title": "mine"}))["title"] == "mine", "owner edits")
        got = ok(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"status": "done"}))
        check(got["owner"] == aid and got["status"] == "done", "admin edit keeps owner")
        err(s.req("DELETE", "/v1/tickets/" + t["id"], a), 403, "forbidden")  # members never delete, even own
        ok(s.req("DELETE", "/v1/tickets/" + t["id"], s.admin))


@test("T-R10-role-before-existence", ["R10", "R3"], "403/404 ordering for missing tickets")
def _(ctx):
    s = main_srv(ctx)
    _, v = mk_token(s, "viewer")
    _, m = mk_token(s, "member")
    err(s.req("PATCH", "/v1/tickets/nosuch", v, {"title": "x"}), 403, "forbidden")
    err(s.req("DELETE", "/v1/tickets/nosuch", v), 403, "forbidden")
    err(s.req("DELETE", "/v1/tickets/nosuch", m), 403, "forbidden")
    err(s.req("PATCH", "/v1/tickets/nosuch", m, {"title": "x"}), 404, "not_found")
    err(s.req("DELETE", "/v1/tickets/nosuch", s.admin), 404, "not_found")
    err(s.req("DELETE", "/v1/tokens/tok_ffffffffffffffff", s.admin), 404, "not_found")
    err(s.req("DELETE", "/v1/tokens/tok_ffffffffffffffff", m), 403, "forbidden")


@test("T-R10-forbidden-no-mutation", ["R10"], "denied operations change nothing")
def _(ctx):
    with ctx.fresh() as s:
        _, v = mk_token(s, "viewer")
        _, m = mk_token(s, "member")
        t = mk_ticket(s, s.admin, title="stay")
        before = ok(s.req("GET", "/v1/tickets", s.admin))
        tokens_before = ok(s.req("GET", "/v1/tokens", s.admin))
        for tok in (v, m):
            s.req("PATCH", "/v1/tickets/" + t["id"], tok, {"title": "evil"})
            s.req("DELETE", "/v1/tickets/" + t["id"], tok)
            s.req("POST", "/v1/tokens", tok, {"name": "evil", "role": "admin"})
        s.req("POST", "/v1/tickets", v, {"title": "evil"})
        check(ok(s.req("GET", "/v1/tickets", s.admin)) == before, "tickets changed by denied calls")
        check(ok(s.req("GET", "/v1/tokens", s.admin)) == tokens_before, "tokens changed by denied calls")


@test("T-R10-privilege-escalation", ["R10", "R7"], "a member cannot mint a stronger token or change roles")
def _(ctx):
    s = main_srv(ctx)
    mid, m = mk_token(s, "member")
    err(s.req("POST", "/v1/tokens", m, {"name": "x", "role": "admin"}), 403, "forbidden")
    err(s.req("DELETE", "/v1/tokens/" + mid, m), 403, "forbidden")
    check(ok(s.req("GET", "/v1/me", m))["role"] == "member", "role unchanged")


# =================================================================== R11 token lifecycle
@test("T-R11-create-token-shape", ["R11", "R3"], "201 shape, formats, Cache-Control: no-store, uniqueness")
def _(ctx):
    s = main_srv(ctx)
    seen_ids, seen_tok = set(), set()
    for i in range(5):
        r = s.req("POST", "/v1/tokens", s.admin, {"name": "ci-%d" % i, "role": "member"})
        j = ok(r, 201)
        check(set(j) == TOKEN_KEYS | {"token"}, "keys %r" % sorted(j), r)
        check(TOKID_RE.match(j["id"]) and SECRET_RE.match(j["token"]), "id/token format", r)
        check((j["name"], j["role"], j["revoked"]) == ("ci-%d" % i, "member", False), "values", r)
        check(type(j["created_at"]) is int and j["created_at"] > 0, "created_at", r)
        check("no-store" in (r.h("cache-control") or "").lower(), "Cache-Control: no-store", r)
        check(j["id"] not in seen_ids and j["token"] not in seen_tok, "uniqueness")
        seen_ids.add(j["id"]); seen_tok.add(j["token"])
        s.secrets.append(j["token"])


@test("T-R11-secret-only-once", ["R11", "R12"], "the secret appears in no later response")
def _(ctx):
    with ctx.fresh() as s:
        tid, tok = mk_token(s, "admin", "once")
        mk_ticket(s, tok)
        texts = [s.req("GET", "/v1/tokens", s.admin).text, s.req("GET", "/v1/me", tok).text, s.req("GET", "/v1/tickets", tok).text,
                 s.req("DELETE", "/v1/tokens/" + tid, s.admin).text, s.req("GET", "/v1/tokens", s.admin).text]
        for t in texts:
            check(tok not in t, "secret appears again: %s" % t[:200])
            check(hashlib_hex(tok) not in t, "hash of the secret appears in a response")


def hashlib_hex(secret):
    return hashlib.sha256(secret.encode("utf-8")).hexdigest()


@test("T-R11-list-tokens", ["R11"], "list: creation order, revoked included, no bootstrap, no secrets")
def _(ctx):
    with ctx.fresh() as s:
        check(ok(s.req("GET", "/v1/tokens", s.admin)) == {"items": []}, "empty list with only the bootstrap token")
        made = [mk_token(s, role, "n" + role) for role in ("admin", "member", "viewer")]
        ok(s.req("DELETE", "/v1/tokens/" + made[1][0], s.admin))
        j = ok(s.req("GET", "/v1/tokens", s.admin))
        check(set(j) == {"items"}, "list keys")
        check([i["id"] for i in j["items"]] == [m[0] for m in made], "creation order, no bootstrap")
        for it in j["items"]:
            check(set(it) == TOKEN_KEYS, "item keys %r" % sorted(it))
        check([i["revoked"] for i in j["items"]] == [False, True, False], "revoked flags")
        check([i["role"] for i in j["items"]] == ["admin", "member", "viewer"], "roles")


@test("T-R11-revoke-effect", ["R11", "R9"], "revoked token gets 401 on the very next request; revoke is idempotent")
def _(ctx):
    with ctx.fresh() as s:
        tid, tok = mk_token(s, "member")
        ok(s.req("GET", "/v1/me", tok))
        r = s.req("DELETE", "/v1/tokens/" + tid, s.admin)
        j = ok(r)
        check(set(j) == TOKEN_KEYS and j["revoked"] is True and j["id"] == tid, "revoke response")
        err(s.req("GET", "/v1/me", tok), 401, "unauthorized")
        err(s.req("POST", "/v1/tickets", tok, {"title": "x"}), 401, "unauthorized")
        check(ok(s.req("DELETE", "/v1/tokens/" + tid, s.admin))["revoked"] is True, "second revoke is 200")


@test("T-R11-revoke-unknown-and-bootstrap", ["R11", "R8"], "unknown id 404; bootstrap 409")
def _(ctx):
    s = main_srv(ctx)
    err(s.req("DELETE", "/v1/tokens/tok_0000000000000000", s.admin), 404, "not_found")
    err(s.req("DELETE", "/v1/tokens/bootstrap", s.admin), 409, "conflict")
    ok(s.req("GET", "/v1/me", s.admin))
    aid, a = mk_token(s, "admin")
    err(s.req("DELETE", "/v1/tokens/bootstrap", a), 409, "conflict")


@test("T-R11-admin-token-powers", ["R11", "R10"], "an issued admin token manages tokens, may revoke itself")
def _(ctx):
    with ctx.fresh() as s:
        aid, a = mk_token(s, "admin")
        bid, b = mk_token(s, "viewer")
        j = ok(s.req("POST", "/v1/tokens", a, {"name": "child", "role": "member"}), 201)
        ok(s.req("DELETE", "/v1/tokens/" + j["id"], a))
        ok(s.req("DELETE", "/v1/tokens/" + aid, a))
        err(s.req("GET", "/v1/tokens", a), 401, "unauthorized")
        ok(s.req("GET", "/v1/me", b))
        ok(s.req("GET", "/v1/tokens", s.admin))


@test("T-R11-tickets-outlive-token", ["R11", "R5"], "tickets stay with their owner id after revocation")
def _(ctx):
    with ctx.fresh() as s:
        mid, m = mk_token(s, "member")
        t = mk_ticket(s, m, title="orphan")
        ok(s.req("DELETE", "/v1/tokens/" + mid, s.admin))
        g = ok(s.req("GET", "/v1/tickets/" + t["id"], s.admin))
        check(g["owner"] == mid, "owner retained")
        ok(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"status": "done"}))


# =================================================================== R12 secrets
def scan_state(s, secrets, extra_forbidden=()):
    blobs = s.state_blobs()
    check(blobs, "no state file found under the data directory (%s)" % s.dir)
    for name, data in blobs.items():
        for sec in list(secrets) + list(extra_forbidden):
            check(sec.encode("utf-8") not in data, "secret %s... found in state file %s" % (sec[:8], name))
    return b"\n".join(blobs.values())


@test("T-R12-secrets-not-in-state", ["R12", "R13"], "no token secret or Authorization value in any state file; SHA-256 hex is there")
def _(ctx):
    with ctx.fresh() as s:
        toks = [mk_token(s, r)[1] for r in ("admin", "member", "viewer", "member")]
        mk_ticket(s, toks[1], title="x")
        s.req("GET", "/v1/tickets", auth="Bearer wrong-secret-value-123")
        s.req("POST", "/v1/tickets", toks[2], {"title": "denied"})
        all_bytes = scan_state(s, toks + [s.admin], ["wrong-secret-value-123", "Bearer " + s.admin])
        for tk in toks:
            check(hashlib_hex(tk).encode() in all_bytes, "lowercase hex SHA-256 of an issued secret must be in the state")
        s.restart()
        scan_state(s, toks + [s.admin])
        ok(s.req("GET", "/v1/me", toks[0]))
        s.restart(sig=signal.SIGKILL)
        ok(s.req("POST", "/v1/tickets", toks[1], {"title": "after kill"}), 201)
        scan_state(s, toks + [s.admin])


@test("T-R12-secrets-not-on-stderr", ["R12", "R16"], "no secret or Authorization value on stderr/stdout after a mixed workload")
def _(ctx):
    s = main_srv(ctx)
    tid, tok = mk_token(s, "member")
    s.req("GET", "/v1/tickets", tok)
    s.req("POST", "/v1/tickets", tok, "{bad json")
    s.req("POST", "/v1/tickets", tok, {"title": ""})
    s.req("GET", "/v1/tickets", auth="Bearer secret-wrong-" + "q" * 20)
    s.req("DELETE", "/v1/tokens/" + tid, s.admin)
    s.req("GET", "/v1/tickets", tok)
    time.sleep(0.3)
    secrets = [s.admin, tok, "secret-wrong-" + "q" * 20] + s.secrets
    for stream, text in (("stderr", s.stderr()), ("stdout", s.stdout())):
        for sec in secrets:
            check(sec not in text, "secret leaked on %s: %s..." % (stream, sec[:8]))
        check("Bearer " not in text, "Authorization value on " + stream)


# =================================================================== R13 persistence
def snapshot(s):
    _, items = walk(s, s.admin, limit=100)
    return items, ok(s.req("GET", "/v1/tokens", s.admin))["items"]


@test("T-R13-restart-sigterm", ["R13"], "tickets, order and tokens identical after SIGTERM restart")
def _(ctx):
    with ctx.fresh() as s:
        mid, m = mk_token(s, "member")
        vid, v = mk_token(s, "viewer")
        ok(s.req("DELETE", "/v1/tokens/" + vid, s.admin))
        a = mk_ticket(s, s.admin, title="alpha", body="b1", priority="high")
        b = mk_ticket(s, m, title="beta \U0001F525")
        ok(s.req("PATCH", "/v1/tickets/" + a["id"], s.admin, {"status": "done"}))
        c = mk_ticket(s, s.admin, title="gamma")
        ok(s.req("DELETE", "/v1/tickets/" + c["id"], s.admin))
        before = snapshot(s)
        s.restart()
        check(snapshot(s) == before, "state differs after restart")
        check(ok(s.req("GET", "/v1/me", m))["id"] == mid, "issued token still valid")
        err(s.req("GET", "/v1/me", v), 401, "unauthorized")
        ok(s.req("PATCH", "/v1/tickets/" + b["id"], m, {"title": "beta2"}))


@test("T-R13-sigkill-durability", ["R13"], "confirmed mutations survive SIGKILL immediately after the response")
def _(ctx):
    with ctx.fresh() as s:
        ids = []
        for i in range(5):
            ids.append(mk_ticket(s, s.admin, title="k%d" % i)["id"])
        tid, tok = mk_token(s, "member")
        ok(s.req("PATCH", "/v1/tickets/" + ids[0], s.admin, {"title": "patched"}))
        ok(s.req("DELETE", "/v1/tickets/" + ids[1], s.admin))
        before = snapshot(s)
        s.restart(sig=signal.SIGKILL)
        check(snapshot(s) == before, "state lost across SIGKILL")
        ok(s.req("GET", "/v1/me", tok))


@test("T-R13-id-no-reuse-after-restart", ["R13", "R5"], "ids of deleted tickets are not reused after restart")
def _(ctx):
    with ctx.fresh() as s:
        seen = [mk_ticket(s, s.admin, title="i%d" % i)["id"] for i in range(4)]
        ok(s.req("DELETE", "/v1/tickets/" + seen[-1], s.admin))
        ok(s.req("DELETE", "/v1/tickets/" + seen[0], s.admin))
        s.restart()
        new = [mk_ticket(s, s.admin)["id"] for _ in range(3)]
        check(not set(new) & set(seen) and len(set(new)) == 3, "reused ids: %r vs %r" % (new, seen))
        _, items = walk(s, s.admin)
        check([t["id"] for t in items][-3:] == new, "new tickets sort after old ones")


@test("T-R13-bootstrap-rotation", ["R13", "R9"], "restart with a new bootstrap token: new accepted, old rejected, issued tokens kept")
def _(ctx):
    with ctx.fresh() as s:
        old = s.admin
        mid, m = mk_token(s, "member")
        t = mk_ticket(s, m)
        new = "rotated-" + os.urandom(12).hex()
        s.restart(admin=new)
        ok(s.req("GET", "/v1/me", new))
        err(s.req("GET", "/v1/me", old), 401, "unauthorized")
        check(ok(s.req("GET", "/v1/tickets/" + t["id"], m))["owner"] == mid, "data and issued token intact")
        scan_state(s, [old, new, m])


@test("T-R13-state-in-granted-directory", ["R13", "R2"], "state lives at TICKETDESK_DATA's directory only")
def _(ctx):
    d = ctx.tmp()
    inner = os.path.join(d, "data")
    os.mkdir(inner)
    with ctx.fresh(data=os.path.join(inner, "s.state"), data_dir=d) as s:
        mk_ticket(s, s.admin)
        s.stop()
        names = []
        for root, _dirs, files in os.walk(d):
            names += [os.path.relpath(os.path.join(root, f), d) for f in files if not f.startswith(("stdout.", "stderr."))]
        check(names, "no state written")
        check(all(n.startswith("data" + os.sep) for n in names), "files written outside the data directory: %r" % names)


@test("T-R13-concurrent-no-lost-updates", ["R13", "R17"], "parallel PATCHes and creates lose nothing")
def _(ctx):
    with ctx.fresh() as s:
        t = mk_ticket(s, s.admin, title="shared")
        errs, ids = [], []
        lk = threading.Lock()

        def worker(i):
            try:
                r = s.req("POST", "/v1/tickets", s.admin, {"title": "c%d" % i})
                with lk:
                    ids.append(ok(r, 201)["id"])
                for st in ("in_progress", "done"):
                    ok(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, {"status": st}))
            except Exception as e:  # noqa: BLE001
                with lk:
                    errs.append(repr(e))
        ths = [threading.Thread(target=worker, args=(i,)) for i in range(10)]
        [x.start() for x in ths]
        [x.join(60) for x in ths]
        check(not errs, "worker errors: %r" % errs[:2])
        check(len(set(ids)) == 10, "ids must be unique under concurrency")
        _, items = walk(s, s.admin, limit=100)
        check(len(items) == 11, "expected 11 tickets, got %d" % len(items))
        s.restart()
        _, items2 = walk(s, s.admin, limit=100)
        check(items2 == items, "concurrent state not durable")


# =================================================================== R14 body ceiling
@test("T-R14-default-ceiling", ["R14"], "default 1048576: N bytes processed, N+1 rejected with 413")
def _(ctx):
    s = main_srv(ctx)
    n = 1048576
    at = "{" + " " * (n - 2) + "}"
    check(len(at) == n, "test bug")
    err(s.req("POST", "/v1/tickets", s.admin, at), 400, "validation_failed", {("title", "required")})
    err(s.req("POST", "/v1/tickets", s.admin, at + " "), 413, "payload_too_large")
    err(s.req("POST", "/v1/tickets", s.admin, b"x" * (n + 1000)), 413, "payload_too_large")
    t = mk_ticket(s, s.admin)
    err(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, b" " * (n + 1)), 413, "payload_too_large")
    err(s.req("POST", "/v1/tokens", s.admin, b" " * (n + 1)), 413, "payload_too_large")
    ok(s.req("GET", "/v1/health"))


@test("T-R14-custom-ceiling", ["R14"], "--max-request-bytes 2048: exact boundary on POST/PATCH/tokens")
def _(ctx):
    with ctx.fresh(flags="--max-request-bytes 2048") as s:
        def pad(prefix, suffix, n):
            return prefix + "a" * (n - len(prefix) - len(suffix)) + suffix
        b = pad('{"title":"x","body":"', '"}', 2048)
        check(len(b.encode()) == 2048, "test bug")
        ok(s.req("POST", "/v1/tickets", s.admin, b), 201)
        err(s.req("POST", "/v1/tickets", s.admin, pad('{"title":"x","body":"', '"}', 2049)), 413, "payload_too_large")
        t = mk_ticket(s, s.admin)
        ok(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, pad('{"body":"', '"}', 2048)))
        err(s.req("PATCH", "/v1/tickets/" + t["id"], s.admin, pad('{"body":"', '"}', 2049)), 413, "payload_too_large")
        err(s.req("POST", "/v1/tokens", s.admin, pad('{"name":"n","role":"viewer","x":"', '"}', 2049)), 413, "payload_too_large")
        check(ok(s.req("GET", "/v1/tickets/" + t["id"], s.admin))["body"] == "a" * (2048 - len('{"body":"') - 2), "patched body intact")
        ok(s.req("POST", "/v1/tickets", s.admin, {"title": "still fine"}), 201)


@test("T-R14-ceiling-before-json", ["R14", "R8"], "oversize non-JSON is 413, not invalid_json; multibyte counted in bytes")
def _(ctx):
    with ctx.fresh(flags="--max-request-bytes 1000") as s:
        err(s.req("POST", "/v1/tickets", s.admin, b"\xff" * 1001), 413, "payload_too_large")
        err(s.req("POST", "/v1/tickets", s.admin, "not json " * 200), 413, "payload_too_large")
        body = '{"title":"x","body":"' + "é" * 500 + '"}'
        check(len(body.encode()) > 1000, "test bug")
        err(s.req("POST", "/v1/tickets", s.admin, body), 413, "payload_too_large")
        ok(s.req("GET", "/v1/me", s.admin))
        err(s.req("POST", "/v1/tickets", None, b"x" * 2000), 401, "unauthorized")  # auth first


@test("T-R14-invalid-flag-values", ["R14"], "--max-request-bytes 0 / -5 / abc: runner exits 2 with a diagnostic")
def _(ctx):
    for bad in ("0", "-5", "abc"):
        res, _s = ctx.try_start(flags="--max-request-bytes " + bad, timeout=15)
        check(not res.hung, "must not keep running with an invalid flag value", res)
        check(res.rc == 2, "expected exit 2, got %r" % res.rc, res)
        check(res.err.strip(), "diagnostic on stderr expected", res)


# =================================================================== R15 fault handling
def fault_srv(ctx):
    return ctx.cache("fault", flags="--access-log", env={"TICKETDESK_ALLOW_DEBUG_FAULT": "1"})


def fault_check(r):
    err(r, 500, "internal_error")
    check(r.json["error"]["message"] == INTERNAL_MSG, "fixed 500 message, got %r" % r.json["error"]["message"], r)
    no_internals(r.text)
    check(set(r.json["error"]) == {"code", "message", "request_id"}, "500 body must have only code/message/request_id", r)


@test("T-R15-fault-shape", ["R15", "R8"], "forced fault: safe 500, request id, no internals in body or headers")
def _(ctx):
    s = fault_srv(ctx)
    r = s.req("GET", "/v1/health", headers=[("X-Debug-Fault", "1")])
    fault_check(r)
    for k, v in r.hlist:
        no_internals(k + ": " + v)
    check(r.h("x-request-id") == r.json["error"]["request_id"], "ids match", r)


@test("T-R15-fault-stderr-line", ["R15", "R16"], "stderr has 'fault request_id=... method=... path=... error=...' with the same id")
def _(ctx):
    s = fault_srv(ctx)
    tid, tok = mk_token(s, "member")
    r = s.req("POST", "/v1/tickets?a=b&c=d", tok, {"title": "boom"}, headers=[("X-Debug-Fault", "1")])
    fault_check(r)
    rid = r.json["error"]["request_id"]

    def find():
        for ln in stderr_lines(s):
            m = FAULT_RE.match(ln)
            if m and m.group(1) == rid:
                return m
    m = wait_for(find)
    check(m, "no fault line for request_id %s on stderr:\n%s" % (rid, s.stderr()[-1500:]))
    check(m.group(2) == "POST" and m.group(3) == "/v1/tickets", "method/path in fault line (query stripped): %r" % m.group(0))
    check(len(m.group(4).strip()) >= 3 and tok not in m.group(0), "error text non-empty, no secret")
    check(sum(1 for ln in stderr_lines(s) if ln.startswith("fault ") and rid in ln) == 1, "exactly one fault line per fault")
    check("Bearer" not in s.stderr() and tok not in s.stderr() and s.admin not in s.stderr(), "secrets on stderr")


@test("T-R15-fault-every-route", ["R15"], "fault honored on every known path/method, with and without auth")
def _(ctx):
    s = fault_srv(ctx)
    for m, p in [("GET", "/v1/health"), ("GET", "/v1/me"), ("GET", "/v1/tickets"), ("POST", "/v1/tickets"), ("GET", "/v1/tickets/x"),
                 ("PATCH", "/v1/tickets/x"), ("DELETE", "/v1/tickets/x"), ("GET", "/v1/tokens"), ("POST", "/v1/tokens"),
                 ("DELETE", "/v1/tokens/x"), ("PUT", "/v1/tickets")]:
        for tok in (None, s.admin):
            fault_check(s.req(m, p, tok, {"title": "x"} if m in ("POST", "PATCH") else None, headers=[("X-Debug-Fault", "1")]))


@test("T-R15-fault-no-side-effects-and-recovery", ["R15", "R13"], "after faults the server serves normally and state is unchanged")
def _(ctx):
    with ctx.fresh(flags="--access-log", env={"TICKETDESK_ALLOW_DEBUG_FAULT": "1"}) as s:
        t = mk_ticket(s, s.admin, title="before")
        before = snapshot(s)
        for _ in range(5):
            fault_check(s.req("POST", "/v1/tickets", s.admin, {"title": "ghost"}, headers=[("X-Debug-Fault", "1")]))
            fault_check(s.req("DELETE", "/v1/tickets/" + t["id"], s.admin, headers=[("X-Debug-Fault", "1")]))
        check(snapshot(s) == before, "a faulting request must not mutate state")
        ok(s.req("POST", "/v1/tickets", s.admin, {"title": "after"}), 201)
        check(s.proc.poll() is None, "server alive")


@test("T-R15-fault-header-value", ["R15"], "only the value 1 triggers; other values and unknown routes are unaffected")
def _(ctx):
    s = fault_srv(ctx)
    for v in ("0", "true", "", "2", "yes"):
        ok(s.req("GET", "/v1/health", headers=[("X-Debug-Fault", v)]))
    err(s.req("GET", "/v1/nope", headers=[("X-Debug-Fault", "1")]), 404, "not_found")


@test("T-R15-fault-disabled-by-default", ["R15", "R9"], "without TICKETDESK_ALLOW_DEBUG_FAULT=1 the header is inert")
def _(ctx):
    s = main_srv(ctx)
    ok(s.req("GET", "/v1/health", headers=[("X-Debug-Fault", "1")]))
    ok(s.req("GET", "/v1/me", s.admin, headers=[("X-Debug-Fault", "1")]))
    for val in ("true", "0", "", "yes", " 1", "11"):
        with ctx.fresh(env={"TICKETDESK_ALLOW_DEBUG_FAULT": val}) as f:
            ok(f.req("GET", "/v1/health", headers=[("X-Debug-Fault", "1")]))
            ok(f.req("POST", "/v1/tickets", f.admin, {"title": "x"}, headers=[("X-Debug-Fault", "1")]), 201)


# =================================================================== R16 observability
@test("T-R16-access-log-lines", ["R16"], "--access-log: one exact line per request incl. 401/404/405/413, path without query")
def _(ctx):
    with ctx.fresh(flags="--access-log --max-request-bytes 4096") as s:
        _, v = mk_token(s, "viewer")
        reqs = [
            (s.req("GET", "/v1/health"), "GET", "/v1/health", 200),
            (s.req("GET", "/v1/tickets?limit=2&status=open", s.admin), "GET", "/v1/tickets", 200),
            (s.req("POST", "/v1/tickets", s.admin, {"title": "log"}), "POST", "/v1/tickets", 201),
            (s.req("POST", "/v1/tickets", s.admin, {}), "POST", "/v1/tickets", 400),
            (s.req("GET", "/v1/tickets"), "GET", "/v1/tickets", 401),
            (s.req("POST", "/v1/tickets", v, {"title": "x"}), "POST", "/v1/tickets", 403),
            (s.req("GET", "/v1/nothing?q=1"), "GET", "/v1/nothing", 404),
            (s.req("PUT", "/v1/tickets"), "PUT", "/v1/tickets", 405),
            (s.req("POST", "/v1/tickets", s.admin, b" " * 5000), "POST", "/v1/tickets", 413),
            (s.req("POST", "/v1/tickets", s.admin, {"title": "x"}, ctype="text/plain"), "POST", "/v1/tickets", 415),
            (s.req("DELETE", "/v1/tokens/bootstrap", s.admin), "DELETE", "/v1/tokens/bootstrap", 409),
        ]
        time.sleep(0.2)
        for r, method, path, status in reqs:
            rid = r.h("x-request-id")
            check(r.status == status, "setup: %s %s expected %d got %d" % (method, path, status, r.status), r)

            def find(rid=rid):
                return [ln for ln in stderr_lines(s) if ("request_id=" + rid) in ln and ln.startswith("access ")]
            lines = wait_for(find)
            check(len(lines) == 1, "want exactly one access line for %s, got %r\n%s" % (rid, lines, s.stderr()[-1500:]))
            m = ACCESS_RE.match(lines[0])
            check(m, "access line does not match the exact format: %r" % lines[0])
            check((m.group(1), m.group(2), int(m.group(3))) == (method, path, status), "access line fields: %r" % lines[0])
        n_access = sum(1 for ln in stderr_lines(s) if ln.startswith("access "))
        check(n_access >= len(reqs), "access lines present")
        check("limit=2" not in s.stderr() and "?" not in "".join(ln for ln in stderr_lines(s) if ln.startswith("access ")),
              "query string must not appear in access lines")


@test("T-R16-access-log-secrets", ["R16", "R12"], "access lines never contain tokens or Authorization")
def _(ctx):
    s = main_srv(ctx)
    tid, tok = mk_token(s, "member")
    s.req("GET", "/v1/tickets/none", tok)
    s.req("GET", "/v1/tickets?cursor=" + tok, tok)
    time.sleep(0.3)
    lines = [ln for ln in stderr_lines(s) if ln.startswith("access ")]
    check(lines and all(ACCESS_RE.match(ln) for ln in lines), "all access lines match the format")
    check(all(tok not in ln and s.admin not in ln for ln in lines), "secret in access line")


@test("T-R16-no-access-without-flag", ["R16"], "without --access-log no 'access ' lines")
def _(ctx):
    with ctx.fresh() as s:
        for i in range(5):
            s.req("GET", "/v1/health")
            s.req("GET", "/v1/tickets", s.admin)
            s.req("GET", "/v1/nope")
        time.sleep(0.3)
        bad = [ln for ln in stderr_lines(s) if ln.startswith("access ")]
        check(not bad, "access lines without the flag: %r" % bad[:2])


@test("T-R16-fault-line-without-flag", ["R16", "R15"], "fault line is written even without --access-log")
def _(ctx):
    with ctx.fresh(env={"TICKETDESK_ALLOW_DEBUG_FAULT": "1"}) as s:
        r = s.req("GET", "/v1/me", s.admin, headers=[("X-Debug-Fault", "1")])
        fault_check(r)
        rid = r.json["error"]["request_id"]
        m = wait_for(lambda: [ln for ln in stderr_lines(s) if ln.startswith("fault request_id=" + rid)])
        check(m and FAULT_RE.match(m[0]), "fault line missing or malformed: %r" % m)
        check(not [ln for ln in stderr_lines(s) if ln.startswith("access ")], "no access lines without the flag")


@test("T-R16-access-line-for-fault", ["R16", "R15"], "a faulting request also gets an access line with status=500")
def _(ctx):
    s = fault_srv(ctx)
    r = s.req("GET", "/v1/tickets", s.admin, headers=[("X-Debug-Fault", "1")])
    rid = r.json["error"]["request_id"]
    lines = wait_for(lambda: [ln for ln in stderr_lines(s) if ln.startswith("access ") and "request_id=" + rid in ln])
    check(len(lines) == 1 and ACCESS_RE.match(lines[0]) and " status=500 " in lines[0], "access line for the fault: %r" % lines)


# =================================================================== R17 robustness
@test("T-R17-concurrency-smoke", ["R17", "R5"], "20 sequential then 20 parallel creates/reads: all correct, unique ids, healthy")
def _(ctx):
    with ctx.fresh(flags="--access-log") as s:
        ids = []
        for i in range(20):
            ids.append(mk_ticket(s, s.admin, title="seq%d" % i)["id"])
            ok(s.req("GET", "/v1/tickets/" + ids[-1], s.admin))
        errors, pids, lk = [], [], threading.Lock()

        def worker(i):
            try:
                r = s.req("POST", "/v1/tickets", s.admin, {"title": "par%d" % i})
                j = ok(r, 201)
                g = ok(s.req("GET", "/v1/tickets/" + j["id"], s.admin))
                check(g["title"] == "par%d" % i, "read-your-write")
                ok(s.req("GET", "/v1/tickets?limit=5", s.admin))
                with lk:
                    pids.append(j["id"])
            except Exception as e:  # noqa: BLE001
                with lk:
                    errors.append(repr(e)[:300])
        ths = [threading.Thread(target=worker, args=(i,)) for i in range(20)]
        [t.start() for t in ths]
        [t.join(90) for t in ths]
        check(not errors, "parallel errors: %r" % errors[:2])
        check(len(set(pids + ids)) == 40, "all 40 ids unique")
        _, items = walk(s, s.admin, limit=100)
        check(len(items) == 40, "all tickets listed, got %d" % len(items))
        check(s.proc.poll() is None and ok(s.req("GET", "/v1/health")) == {"status": "ok"}, "server healthy")


@test("T-R17-max-size-roundtrip", ["R17", "R7"], "largest valid ticket (120-char title, 4000-char body) round-trips")
def _(ctx):
    s = main_srv(ctx)
    t = mk_ticket(s, s.admin, title="T" * 120, body="B" * 4000)
    check(ok(s.req("GET", "/v1/tickets/" + t["id"], s.admin)) == t, "round trip")
    _, items = walk(s, s.admin, limit=100)
    check(t in items, "present in list")


@test("T-R17-error-storm-survival", ["R17", "R8"], "50 hostile requests in a row; server stays healthy and consistent")
def _(ctx):
    with ctx.fresh() as s:
        t = mk_ticket(s, s.admin, title="anchor")
        hostile = [("POST", "/v1/tickets", b"\x00\x01\x02"), ("POST", "/v1/tickets", "[" * 5000), ("POST", "/v1/tickets", '{"title":' + '"' * 3),
                   ("PATCH", "/v1/tickets/" + t["id"], '{"title":{"a":{"b":[1,2,3]}}}'), ("POST", "/v1/tokens", '{"name":"' + "x" * 100000 + '"}'),
                   ("GET", "/v1/tickets?limit=" + "9" * 5000, None), ("GET", "/v1/tickets/" + "z" * 5000, None)]
        for i in range(50):
            m, p, b = hostile[i % len(hostile)]
            r = s.req(m, p, s.admin, b)
            check(400 <= r.status < 500 and r.json and "error" in r.json, "hostile request %d got %s" % (i, r.status), r)
        check(ok(s.req("GET", "/v1/tickets/" + t["id"], s.admin)) == t, "anchor unchanged")
        check(s.proc.poll() is None, "server alive")


if __name__ == "__main__":
    sys.exit(main())
