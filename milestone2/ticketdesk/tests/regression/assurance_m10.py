#!/usr/bin/env python3
"""Assurance regression tests for the Milestone 2 suite blind spots listed as m10 in REVIEW.md:
port-in-use stderr shape, default-flag operation, double slashes in paths, slow-body clients.

Same conventions as review_round1.py: the server is started through the frozen launcher with the DEFAULT
runner ceilings, one fresh data directory per test, Python 3 standard library only.

Usage: HF_BIN=... APP_ARTIFACT=... assurance_m10.py LAUNCHER

The bootstrap token is generated at runtime and is never printed. A test listed in SKIPPED records a
finding at the reviewed pin (see milestone2/ASSURANCE_M2_FINAL_REVIEW.md); it is reported as SKIP with the
finding id. Set M10_RUN_SKIPPED=1 to run those tests anyway (they then count as failures if they fail).
"""
import fcntl
import http.client
import json
import os
import secrets
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time

L = os.path.abspath(sys.argv[1])
ADMIN = "m10-" + secrets.token_hex(24)  # runtime-only test secret, never logged
FORBIDDEN_STDERR = ('"phase"', "Traceback", "panic:", "goroutine ", "RUNTIME_ERROR", "IO_ERROR", "{")


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    p = s.getsockname()[1]
    s.close()
    return p


def check(cond, msg):
    if not cond:
        raise AssertionError(msg)


class Srv:
    """One server through the launcher. flags=None means no runner flags at all (HF_RUN_FLAGS unset)."""

    def __init__(self, data=None, port=None, flags=None, env_extra=None, stdout=subprocess.PIPE, default_port=False):
        self.dir = tempfile.mkdtemp(prefix="td-m10-")
        self.data = data or os.path.join(self.dir, "state.json")
        self.port = 8080 if default_port else (port or free_port())
        d = os.path.dirname(self.data)
        env = dict(os.environ, TICKETDESK_ADMIN_TOKEN=ADMIN, TICKETDESK_DATA=self.data)
        env.pop("HF_RUN_FLAGS", None)
        env.pop("TICKETDESK_ALLOW_DEBUG_FAULT", None)
        env.pop("TICKETDESK_PORT", None)
        if not default_port:
            env["TICKETDESK_PORT"] = str(self.port)
        if flags is not None:
            env["HF_RUN_FLAGS"] = flags
        env.update(env_extra or {})
        grants = "network,database,environment,filesystem:read=%s,filesystem:write=%s" % (d, d)
        self.p = subprocess.Popen([L, grants], env=env, stdout=stdout, stderr=subprocess.PIPE)

    def wait(self):
        for _ in range(100):
            try:
                self.req("GET", "/v1/health", timeout=1)
                return self
            except Exception:
                if self.p.poll() is not None:
                    break
                time.sleep(0.1)
        raise AssertionError("server did not start: %r" % self.p.stderr.read()[:300])

    def req(self, method, path, token=None, body=None, timeout=10, raw_body=None, headers=None):
        c = http.client.HTTPConnection("127.0.0.1", self.port, timeout=timeout)
        h = dict(headers or {})
        if token:
            h["Authorization"] = "Bearer " + token
        data = raw_body
        if body is not None:
            data = json.dumps(body).encode()
        if data is not None:
            h.setdefault("Content-Type", "application/json")
        c.request(method, path, body=data, headers=h)
        r = c.getresponse()
        raw = r.read()
        hdrs = {k.lower(): v for k, v in r.getheaders()}
        c.close()
        return r.status, (json.loads(raw) if raw[:1] in (b"{", b"[") else raw), hdrs

    def raw(self, method, path, token=None, body=None, timeout=10):
        """Send the request line exactly as given (no client-side path normalisation)."""
        c = socket.create_connection(("127.0.0.1", self.port), timeout=timeout)
        h = "%s %s HTTP/1.1\r\nHost: x\r\nConnection: close\r\n" % (method, path)
        if token:
            h += "Authorization: Bearer %s\r\n" % token
        data = b""
        if body is not None:
            data = json.dumps(body).encode()
            h += "Content-Type: application/json\r\nContent-Length: %d\r\n" % len(data)
        c.sendall(h.encode() + b"\r\n" + data)
        buf = b""
        while True:
            chunk = c.recv(65536)
            if not chunk:
                break
            buf += chunk
        c.close()
        head, _, payload = buf.partition(b"\r\n\r\n")
        lines = head.decode("latin-1").split("\r\n")
        hdrs = {}
        for line in lines[1:]:
            k, _, v = line.partition(":")
            hdrs[k.strip().lower()] = v.strip()
        try:
            payload = json.loads(payload)
        except ValueError:
            pass
        return int(lines[0].split(" ")[1]), payload, hdrs

    def token(self, role, name=None):
        st, t, _ = self.req("POST", "/v1/tokens", ADMIN, {"name": name or role, "role": role})
        check(st == 201, "token create %s -> %s" % (role, st))
        return t["id"], t["token"]

    def stop(self):
        if self.p.poll() is None:
            self.p.terminate()
            try:
                self.p.wait(5)
            except subprocess.TimeoutExpired:
                self.p.kill()
                self.p.wait(5)
        shutil.rmtree(self.dir, ignore_errors=True)


def stderr_is_clean(err, port):
    check(err.endswith("\n"), "stderr must end with a newline: %r" % err)
    lines = err[:-1].split("\n")
    check(all(l.startswith("ticketdesk: ") for l in lines), "every stderr line must start with 'ticketdesk: ': %r" % err)
    for bad in FORBIDDEN_STDERR:
        check(bad not in err, "stderr contains %r (raw runtime error or trace): %r" % (bad, err))
    check(str(port) in err, "stderr must name the port %d: %r" % (port, err))


def seeded_state():
    """A state file written by the server itself, so 'not modified' compares real bytes."""
    s = Srv().wait()
    try:
        st, _, _ = s.req("POST", "/v1/tickets", ADMIN, {"title": "seed"})
        check(st == 201, "seed create -> %s" % st)
    finally:
        s.p.terminate()
        s.p.wait(5)
    d = tempfile.mkdtemp(prefix="td-m10-")
    path = os.path.join(d, "state.json")
    shutil.copyfile(s.data, path)
    shutil.rmtree(s.dir, ignore_errors=True)
    return d, path


# ---------------------------------------------------------------------------------------------- 1. port in use

def m10_port_in_use_stderr_shape():
    """R1.3: port already in use -> nonzero exit within 10 s, only 'ticketdesk: ' lines, no raw runtime error
    object or trace, data file not modified. Busy listener on 0.0.0.0, on 127.0.0.1 only, and on [::]."""
    variants = [("0.0.0.0", socket.AF_INET), ("127.0.0.1", socket.AF_INET)]
    if socket.has_ipv6:
        variants.append(("::", socket.AF_INET6))
    for host, fam in variants:
        d, path = seeded_state()
        before = open(path, "rb").read()
        busy = socket.socket(fam)
        if fam == socket.AF_INET6:
            busy.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 1)
        busy.bind((host, 0))
        busy.listen(1)
        port = busy.getsockname()[1]
        s = Srv(data=path, port=port)
        try:
            t0 = time.time()
            rc = s.p.wait(12)
            err = s.p.stderr.read().decode("utf-8", "replace")
            check(rc != 0, "busy %s: must exit nonzero, got %s" % (host, rc))
            check(time.time() - t0 < 10, "busy %s: exit took %.1f s (R1.3: within 10 s)" % (host, time.time() - t0))
            stderr_is_clean(err, port)
            check(open(path, "rb").read() == before, "busy %s: existing data file was modified by a failed start" % host)
        finally:
            busy.close()
            s.stop()
            shutil.rmtree(d, ignore_errors=True)


def m10_port_lost_race_stderr_shape():
    """R1.3 when the port is taken AFTER the startup probe and before the listener binds (m12). The child's stdout
    is a full pipe, so it blocks on its 'Listening on N' line, which the runtime prints after preflight (and its
    port probe) and immediately before it binds. While it is blocked we take the port, then let it continue."""
    port = free_port()
    r, w = os.pipe()
    fcntl.fcntl(w, 1031, 4096)  # F_SETPIPE_SZ: smallest pipe
    fl = fcntl.fcntl(w, fcntl.F_GETFL)
    fcntl.fcntl(w, fcntl.F_SETFL, fl | os.O_NONBLOCK)
    try:
        while True:
            os.write(w, b"x" * 512)
    except BlockingIOError:
        pass
    fcntl.fcntl(w, fcntl.F_SETFL, fl)
    s = Srv(port=port, stdout=w)
    os.close(w)
    busy = None
    try:
        time.sleep(1.5)
        if s.p.poll() is not None:
            raise AssertionError("server exited before the race window: %r" % s.p.stderr.read()[:300])
        busy = socket.socket()
        busy.bind(("0.0.0.0", port))
        busy.listen(1)
        while os.read(r, 1 << 16):
            pass
        rc = s.p.wait(12)
        err = s.p.stderr.read().decode("utf-8", "replace")
        check(rc != 0, "must exit nonzero, got %s" % rc)
        stderr_is_clean(err, port)
    finally:
        os.close(r)
        if busy:
            busy.close()
        s.stop()


# ---------------------------------------------------------------------------------------------- 2. default flags

def m10_default_flag_operation():
    """The documented server works with NO runner flags at all (no ceilings, no --max-request-bytes, no
    --access-log) and, when 8080 is free, no TICKETDESK_PORT: full CRUD + token workflow at the frozen maxima,
    the default 1048576-byte body ceiling, no access lines, fault header inert, durable across restart."""
    probe = socket.socket()
    try:
        probe.bind(("0.0.0.0", 8080))
        use_default_port = True
    except OSError:
        use_default_port = False
        print("  note: port 8080 busy on this machine; default-port part not checked, explicit port used")
    finally:
        probe.close()
    s = Srv(flags=None, default_port=use_default_port).wait()
    data = s.data
    try:
        st, h, hdr = s.req("GET", "/v1/health", headers={"X-Debug-Fault": "1"})
        check(st == 200 and h == {"status": "ok"}, "health (fault header must be inert by default): %s %r" % (st, h))
        check(len(hdr.get("x-request-id", "")) == 32, "X-Request-Id missing")
        mid, member = s.token("member")
        _, viewer = s.token("viewer")
        st, me, _ = s.req("GET", "/v1/me", member)
        check(st == 200 and me["role"] == "member" and me["id"] == mid, "me: %s %r" % (st, me))
        big = {"title": "\u00e9" * 120, "body": "\u00e9" * 4000, "priority": "high"}
        st, t, hdr = s.req("POST", "/v1/tickets", member, big)
        check(st == 201 and t["owner"] == mid and hdr.get("location") == "/v1/tickets/" + t["id"],
              "largest valid ticket with no runner flags: %s" % st)
        tid = t["id"]
        for i in range(150):
            st, _, _ = s.req("POST", "/v1/tickets", member, {"title": "ticket %d" % i, "body": "x" * 200})
            check(st == 201, "create %d -> %s" % (i, st))
        seen, cursor = [], None
        while True:
            st, page, _ = s.req("GET", "/v1/tickets?limit=100" + ("&cursor=" + cursor if cursor else ""), viewer)
            check(st == 200, "list page -> %s" % st)
            seen += [x["id"] for x in page["items"]]
            cursor = page["next_cursor"]
            if not cursor:
                break
        check(len(seen) == 151 and len(set(seen)) == 151 and seen[0] == tid, "list walk saw %d" % len(seen))
        st, t2, _ = s.req("PATCH", "/v1/tickets/" + tid, member, {"status": "done", "body": "\u00e9" * 4000})
        check(st == 200 and t2["status"] == "done", "PATCH own max-size ticket: %s" % st)
        st, page, _ = s.req("GET", "/v1/tickets?status=done&limit=1", viewer)
        check(st == 200 and [x["id"] for x in page["items"]] == [tid], "status filter: %s" % st)
        # default body ceiling 1048576 bytes: at the ceiling it is parsed (invalid JSON -> 400), one more is 413
        st, e, _ = s.req("POST", "/v1/tickets", member, raw_body=b"a" * 1048576)
        check(st == 400 and e["error"]["code"] == "invalid_json", "body at default ceiling: %s %r" % (st, e))
        st, e, _ = s.req("POST", "/v1/tickets", member, raw_body=b"a" * 1048577)
        check(st == 413 and e["error"]["code"] == "payload_too_large", "body over default ceiling: %s" % st)
        st, _, _ = s.req("DELETE", "/v1/tickets/" + seen[1], ADMIN)
        check(st == 200, "admin delete -> %s" % st)
    finally:
        s.p.terminate()
        s.p.wait(5)
    err = s.p.stderr.read().decode("utf-8", "replace")
    check("access " not in err, "access lines written without --access-log")
    check(ADMIN not in err and member not in err, "a secret reached stderr")
    s2 = Srv(data=data, flags=None, default_port=use_default_port).wait()
    try:
        st, got, _ = s2.req("GET", "/v1/tickets/" + tid, viewer)
        check(st == 200 and got["status"] == "done" and len(got["body"]) == 4000, "after restart: %s" % st)
        st, _, _ = s2.req("GET", "/v1/tickets/" + seen[1], viewer)
        check(st == 404, "deleted ticket back after restart: %s" % st)
    finally:
        s2.stop()
        shutil.rmtree(s.dir, ignore_errors=True)


# ---------------------------------------------------------------------------------------------- 3. double slashes

def m10_double_slash_routing_and_authz():
    """R3.1/R3.2: paths with doubled slashes are unknown routes (404 not_found, before authentication) for every
    method and caller, and they cannot be used to read, change, create or delete anything."""
    s = Srv().wait()
    try:
        aid, member_a = s.token("member", "a")
        _, member_b = s.token("member", "b")
        vid, viewer = s.token("viewer", "v")
        st, ta, _ = s.req("POST", "/v1/tickets", member_a, {"title": "owned by a"})
        st2, tb, _ = s.req("POST", "/v1/tickets", ADMIN, {"title": "owned by admin"})
        check(st == 201 and st2 == 201, "setup")
        a, b = ta["id"], tb["id"]
        # canonical controls: the rules the aliases must not get around
        check(s.raw("DELETE", "/v1/tickets/" + a, viewer)[0] == 403, "control: viewer DELETE -> 403")
        check(s.raw("PATCH", "/v1/tickets/" + a, member_b, {"title": "x"})[0] == 403, "control: foreign PATCH -> 403")
        check(s.raw("DELETE", "/v1/tickets/" + a, member_a)[0] == 403, "control: member DELETE -> 403")
        check(s.raw("GET", "/v1/tickets")[0] == 401, "control: no token -> 401")
        check(s.raw("GET", "/v1/tokens", member_a)[0] == 403, "control: member token list -> 403")
        st, before, _ = s.req("GET", "/v1/tickets?limit=100", ADMIN)
        st, toks_before, _ = s.req("GET", "/v1/tokens", ADMIN)
        paths = ["//v1/tickets", "/v1//tickets", "///v1/tickets", "/v1/tickets//",
                 "/v1/tickets//" + a, "/v1//tickets/" + a, "//v1/tickets/" + a, "/v1/tickets/" + a + "//",
                 "/v1/tickets//" + a + "/", "/v1/tickets//" + b,
                 "//v1/tokens", "/v1//tokens", "/v1/tokens//" + vid, "//v1/tokens/" + vid, "/v1/tokens/" + vid + "//",
                 "//v1/me", "/v1//me", "//v1/health", "/v1//health"]
        callers = [("none", None), ("viewer", viewer), ("member_b", member_b), ("member_a", member_a), ("admin", ADMIN)]
        bodies = {"POST": {"title": "via alias"}, "PATCH": {"title": "changed via alias", "status": "done"}}
        bad = []
        for path in paths:
            for method in ("GET", "POST", "PATCH", "DELETE"):
                for cname, tok in callers:
                    st, body, hdr = s.raw(method, path, tok, bodies.get(method))
                    code = body.get("error", {}).get("code") if isinstance(body, dict) else None
                    if st != 404 or code != "not_found" or "www-authenticate" in hdr or len(hdr.get("x-request-id", "")) != 32:
                        bad.append("%s %s as %s -> %s %s" % (method, path, cname, st, code))
        check(not bad, "%d alias requests not answered 404 not_found before auth, e.g. %s" % (len(bad), bad[:4]))
        st, after, _ = s.req("GET", "/v1/tickets?limit=100", ADMIN)
        st2, toks_after, _ = s.req("GET", "/v1/tokens", ADMIN)
        check(st == 200 and after == before, "tickets changed through alias paths: %r" % after)
        check(st2 == 200 and toks_after == toks_before, "tokens changed through alias paths")
        check(s.req("GET", "/v1/me", viewer)[0] == 200, "viewer token was revoked through an alias")
    finally:
        s.stop()


# ---------------------------------------------------------------------------------------------- 4. slow clients

class Slow(threading.Thread):
    """A client that trickles one byte per interval and records when the server ends the connection."""

    def __init__(self, port, head, body_total, prefix, interval=1.0, finish=False):
        super().__init__(daemon=True)
        self.port, self.head, self.prefix, self.interval, self.finish = port, head, prefix, interval, finish
        self.body_total = body_total
        self.closed_after = None
        self.response = b""
        self.t0 = None

    def run(self):
        c = socket.create_connection(("127.0.0.1", self.port), timeout=60)
        self.t0 = time.time()
        c.sendall(self.head)
        sent = 0
        c.settimeout(self.interval)
        try:
            while time.time() - self.t0 < 55:
                if sent < len(self.prefix):
                    c.sendall(self.prefix[sent:sent + 1])
                    sent += 1
                elif not self.finish:
                    c.sendall(b" ")  # JSON whitespace: keeps the body incomplete and the client "alive"
                    sent += 1
                try:
                    chunk = c.recv(4096)
                    if chunk == b"" or chunk:
                        self.response += chunk
                        if chunk == b"" or b"\r\n\r\n" in self.response:
                            break
                except socket.timeout:
                    pass
        except OSError:
            pass
        self.closed_after = time.time() - self.t0
        c.close()


def m10_slow_body_clients_bounded():
    """Slow-header and slow-body clients (authenticated, so they reach the body read) are cut off by the server's
    read timeouts, are never executed, and do not delay other clients; a slow but complete upload still works."""
    s = Srv().wait()
    try:
        port = s.port
        auth = "Authorization: Bearer %s\r\n" % ADMIN
        slow_body = [Slow(port, ("POST /v1/tickets HTTP/1.1\r\nHost: x\r\n%sContent-Type: application/json\r\n"
                                 "Content-Length: 200\r\n\r\n" % auth).encode(), 200, b'{"title":"slowloris-body-%d"' % i)
                     for i in range(6)]
        slow_head = [Slow(port, b"GET /v1/health HTTP/1.1\r\nHost: x\r\n", 0, b"X-Slow: " + b"a" * 200) for _ in range(2)]
        ok_body = b'{"title":"slow-but-complete"}'
        slow_ok = Slow(port, ("POST /v1/tickets HTTP/1.1\r\nHost: x\r\n%sContent-Type: application/json\r\n"
                              "Content-Length: %d\r\n\r\n" % (auth, len(ok_body))).encode(), len(ok_body), ok_body,
                       interval=0.12, finish=True)
        clients = slow_body + slow_head + [slow_ok]
        for c in clients:
            c.start()
        time.sleep(1.0)
        worst, created = 0.0, []
        t_end = time.time() + 14
        while time.time() < t_end:
            for method, path, tok, body in (("GET", "/v1/health", None, None), ("GET", "/v1/tickets?limit=5", ADMIN, None),
                                             ("POST", "/v1/tickets", ADMIN, {"title": "probe %d" % len(created)})):
                t0 = time.time()
                try:
                    st, r, _ = s.req(method, path, tok, body, timeout=5)
                except OSError as e:
                    raise AssertionError("%s %s while slow clients were connected: no answer within 5 s (%s)" % (method, path, e))
                worst = max(worst, time.time() - t0)
                check(st in (200, 201), "%s %s during slow clients -> %s" % (method, path, st))
                if st == 201:
                    created.append(r["id"])
            time.sleep(0.5)
        check(worst < 2.0, "a normal request took %.2f s while slow clients were connected" % worst)
        for c in clients:
            c.join(60)
        for c in slow_head:
            check(c.closed_after is not None and c.closed_after < 16,
                  "slow-header client held its connection %.1f s (ReadHeaderTimeout is 10 s)" % (c.closed_after or 99))
        for c in slow_body:
            check(c.closed_after is not None and c.closed_after < 40,
                  "slow-body client held its connection %.1f s (ReadTimeout is 30 s)" % (c.closed_after or 99))
        print("  info: worst normal request %.2f s; slow-header closed after %s s; slow-body closed after %s s" % (
            worst, [round(c.closed_after, 1) for c in slow_head], [round(c.closed_after, 1) for c in slow_body]))
        check(b" 201 " in slow_ok.response.split(b"\r\n", 1)[0], "slow but complete upload: %r" % slow_ok.response[:80])
        st, page, _ = s.req("GET", "/v1/tickets?limit=100", ADMIN)
        titles = [t["title"] for t in page["items"]]
        check(not any(t.startswith("slowloris") for t in titles), "a cut-off slow upload was executed")
        check(len(page["items"]) == len(created) + 1, "list has %d items, want %d" % (len(page["items"]), len(created) + 1))
        st, _, _ = s.req("GET", "/v1/health", timeout=3)
        check(st == 200, "server unhealthy after slow clients")
    finally:
        s.stop()


TESTS = [m10_port_in_use_stderr_shape, m10_port_lost_race_stderr_shape, m10_default_flag_operation,
         m10_double_slash_routing_and_authz, m10_slow_body_clients_bounded]
SKIPPED = {
    "m10_port_lost_race_stderr_shape": "FINDING m12 at pin 7e9ef32e: a port taken between the startup probe and the "
                                       "listen prints the raw IO_ERROR object (ASSURANCE_M2_FINAL_REVIEW.md, m12)",
}
only = set(os.environ.get("M10_ONLY", "").split(",")) - {""}
failed = 0
ran = 0
for t in TESTS:
    if only and t.__name__ not in only:
        continue
    if t.__name__ in SKIPPED and os.environ.get("M10_RUN_SKIPPED") != "1":
        print("SKIP", t.__name__, "-", SKIPPED[t.__name__])
        continue
    ran += 1
    try:
        t()
        print("PASS", t.__name__)
    except Exception as e:
        failed += 1
        print("FAIL", t.__name__, "-", e)
print("\n%d tests, %d failed, %d skipped" % (ran, failed, len([t for t in TESTS if t.__name__ in SKIPPED and os.environ.get("M10_RUN_SKIPPED") != "1" and (not only or t.__name__ in only)])))
sys.exit(1 if failed else 0)
