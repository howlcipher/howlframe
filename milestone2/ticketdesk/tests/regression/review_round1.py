#!/usr/bin/env python3
"""Regression tests for the Milestone 2 review findings B1-B4 (and the state-shape finding).

Unlike the frozen acceptance harness, these runs use the documented command with the DEFAULT runner
ceilings (no raised --max-instructions / --max-memory-bytes). Usage: review_round1.py LAUNCHER
Python 3 standard library only.
"""
import http.client
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time

L = os.path.abspath(sys.argv[1])
ADMIN = "regression-admin-token-0123456789"


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    p = s.getsockname()[1]
    s.close()
    return p


class Srv:
    def __init__(self, data=None, port=None, flags=""):
        self.dir = tempfile.mkdtemp(prefix="td-reg-")
        self.data = data or os.path.join(self.dir, "state.json")
        self.port = port or free_port()
        d = os.path.dirname(self.data)
        env = dict(os.environ, TICKETDESK_ADMIN_TOKEN=ADMIN, TICKETDESK_DATA=self.data, TICKETDESK_PORT=str(self.port),
                   HF_RUN_FLAGS=flags)
        grants = "network,database,environment,filesystem:read=%s,filesystem:write=%s" % (d, d)
        self.p = subprocess.Popen([L, grants], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def wait(self):
        for _ in range(100):
            try:
                self.req("GET", "/v1/health", timeout=1)
                return self
            except Exception:
                if self.p.poll() is not None:
                    break
                time.sleep(0.1)
        raise AssertionError("server did not start: %r" % self.p.stderr.read())

    def req(self, method, path, token=None, body=None, timeout=10):
        c = http.client.HTTPConnection("127.0.0.1", self.port, timeout=timeout)
        h = {}
        if token:
            h["Authorization"] = "Bearer " + token
        data = None
        if body is not None:
            data = json.dumps(body).encode()
            h["Content-Type"] = "application/json"
        c.request(method, path, body=data, headers=h)
        r = c.getresponse()
        raw = r.read()
        c.close()
        return r.status, (json.loads(raw) if raw[:1] in (b"{", b"[") else raw)

    def stop(self):
        if self.p.poll() is None:
            self.p.terminate()
            try:
                self.p.wait(5)
            except subprocess.TimeoutExpired:
                self.p.kill()
        shutil.rmtree(self.dir, ignore_errors=True)


def check(cond, msg):
    if not cond:
        raise AssertionError(msg)


def b1_default_ceilings():
    s = Srv().wait()
    try:
        st, t = s.req("POST", "/v1/tickets", ADMIN, {"title": "t" * 120, "body": "b" * 4000})
        check(st == 201, "largest valid ticket under default ceilings: %s %r" % (st, t))
        for i in range(150):
            st, _ = s.req("POST", "/v1/tickets", ADMIN, {"title": "ticket %d" % i})
            check(st == 201, "create %d -> %s" % (i, st))
        st, page = s.req("GET", "/v1/tickets?limit=1", ADMIN)
        check(st == 200 and len(page["items"]) == 1, "list limit=1 with 151 tickets: %s" % st)
        seen, cursor = 0, None
        while True:
            st, page = s.req("GET", "/v1/tickets?limit=100" + ("&cursor=" + cursor if cursor else ""), ADMIN)
            check(st == 200, "list page -> %s" % st)
            seen += len(page["items"])
            cursor = page["next_cursor"]
            if not cursor:
                break
        check(seen == 151, "walk saw %d of 151" % seen)
        for i in range(30):
            st, _ = s.req("POST", "/v1/tokens", ADMIN, {"name": "n%d" % i, "role": "viewer"})
            check(st == 201, "token create %d -> %s" % (i, st))
        st, toks = s.req("GET", "/v1/tokens", ADMIN)
        check(st == 200 and len(toks["items"]) == 30, "token list: %s" % st)
    finally:
        s.stop()


def b2_port_in_use():
    busy = socket.socket()
    busy.bind(("0.0.0.0", 0))
    busy.listen(1)
    s = Srv(port=busy.getsockname()[1])
    try:
        rc = s.p.wait(15)
        err = s.p.stderr.read().decode()
        check(rc != 0, "must exit nonzero")
        check(err.startswith("ticketdesk: ") and '"phase"' not in err, "stderr must be a prefixed line, got %r" % err)
    finally:
        busy.close()
        s.stop()


def b3_stalled_body_does_not_freeze():
    s = Srv().wait()
    try:
        sock = socket.create_connection(("127.0.0.1", s.port))
        sock.sendall(("POST /v1/tickets HTTP/1.1\r\nHost: x\r\nAuthorization: Bearer %s\r\nContent-Type: application/json\r\n"
                      "Content-Length: 500\r\n\r\n{\"title\":" % ADMIN).encode())
        time.sleep(0.5)
        t0 = time.time()
        st, _ = s.req("GET", "/v1/health", timeout=3)
        check(st == 200 and time.time() - t0 < 2.5, "health blocked by a stalled body")
        st, _ = s.req("GET", "/v1/tickets", ADMIN, timeout=3)
        check(st == 200, "authenticated read blocked by a stalled body")
        sock.close()
    finally:
        s.stop()


def b4_exact_paths():
    s = Srv().wait()
    try:
        for path in ["//v1/tickets", "/v1//tickets", "/v1/tickets/.", "/v1/./tickets", "/v1/x/../tickets", "/v1/tokens//x", "/v1/tickets/"]:
            c = socket.create_connection(("127.0.0.1", s.port))
            c.sendall(("GET %s HTTP/1.1\r\nHost: x\r\nAuthorization: Bearer %s\r\nConnection: close\r\n\r\n" % (path, ADMIN)).encode())
            data = b""
            while True:
                chunk = c.recv(4096)
                if not chunk:
                    break
                data += chunk
            c.close()
            status = int(data.split(b" ", 2)[1])
            check(status == 404, "%s -> %s (want 404 not_found)" % (path, status))
    finally:
        s.stop()


def wrong_shape_state():
    d = tempfile.mkdtemp(prefix="td-reg-")
    path = os.path.join(d, "state.json")
    with open(path, "w") as f:
        json.dump({"meta": {"next_ticket": "x"}}, f)
    s = Srv(data=path)
    try:
        rc = s.p.wait(15)
        err = s.p.stderr.read().decode()
        check(rc == 65 and err.startswith("ticketdesk: "), "wrong-shape meta must exit 65 with a prefixed line, got %s %r" % (rc, err))
    finally:
        s.stop()
        shutil.rmtree(d, ignore_errors=True)


TESTS = [b1_default_ceilings, b2_port_in_use, b3_stalled_body_does_not_freeze, b4_exact_paths, wrong_shape_state]
failed = 0
for t in TESTS:
    try:
        t()
        print("PASS", t.__name__)
    except Exception as e:
        failed += 1
        print("FAIL", t.__name__, "-", e)
print("\n%d tests, %d failed" % (len(TESTS), failed))
sys.exit(1 if failed else 0)
