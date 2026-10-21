#!/usr/bin/env python3
"""Regression for audit finding HF-03: ticketdesk must restart under its documented command
(default instruction and memory ceilings, README flags only) with a realistic amount of state,
and keep every acknowledged record, token and permission across restarts and SIGKILL.

Floor from the remediation mission: at least 1,000 tickets and 100 tokens. The previous
implementation restarted at 1,500 tickets; the replacement failed at about 300.

Usage: HF_BIN=... APP_ARTIFACT=... restart_default_budget.py LAUNCHER
Python 3 standard library only. Every run uses a fresh temporary directory.
"""
import concurrent.futures as cf
import http.client
import json
import os
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time

L = os.path.abspath(sys.argv[1])
ADMIN = "restart-regression-admin-0123456789"
TICKETS, TOKENS = 1000, 100
# The documented command's flags (README), and nothing that raises a ceiling.
DOCUMENTED_FLAGS = "--max-request-bytes 1048576"


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    p = s.getsockname()[1]
    s.close()
    return p


class Srv:
    def __init__(self, workdir, port=None):
        self.dir = workdir
        self.data = os.path.join(workdir, "state.json")
        self.port = port or free_port()
        env = dict(os.environ, TICKETDESK_ADMIN_TOKEN=ADMIN, TICKETDESK_DATA=self.data,
                   TICKETDESK_PORT=str(self.port), HF_RUN_FLAGS=DOCUMENTED_FLAGS)
        grants = "network,database,environment,filesystem:read=%s,filesystem:write=%s" % (workdir, workdir)
        self.p = subprocess.Popen([L, grants], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def wait(self):
        for _ in range(300):
            if self.p.poll() is not None:
                raise AssertionError("server exited %s: %r" % (self.p.returncode, self.p.stderr.read()[:300]))
            try:
                self.req("GET", "/v1/health", timeout=1)
                return self
            except OSError:
                time.sleep(0.1)
        raise AssertionError("server did not become ready")

    def req(self, method, path, token=None, body=None, timeout=15):
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

    def stop(self, sig=signal.SIGTERM):
        if self.p.poll() is None:
            self.p.send_signal(sig)
            try:
                self.p.wait(10)
            except subprocess.TimeoutExpired:
                self.p.kill()
                self.p.wait()


def check(cond, msg):
    if not cond:
        raise AssertionError(msg)


def list_all(s):
    ids, cursor = [], None
    while True:
        st, page = s.req("GET", "/v1/tickets?limit=100" + ("&cursor=" + cursor if cursor else ""), ADMIN)
        check(st == 200, "list page -> %s" % st)
        ids += [t["id"] for t in page["items"]]
        cursor = page["next_cursor"]
        if not cursor:
            return ids


def populate(s):
    """Create TICKETS tickets (owners vary) and TOKENS tokens; return what was acknowledged."""
    tokens = {}  # secret -> (id, role)
    roles = ["admin", "member", "viewer"]
    for i in range(TOKENS):
        role = roles[i % 3]
        st, t = s.req("POST", "/v1/tokens", ADMIN, {"name": "t%d" % i, "role": role})
        check(st == 201, "token %d -> %s" % (i, st))
        tokens[t["token"]] = (t["id"], role)
    members = [sec for sec, (_, r) in tokens.items() if r == "member"]
    tickets = {}
    for i in range(TICKETS):
        owner = members[i % len(members)]
        st, t = s.req("POST", "/v1/tickets", owner, {"title": "ticket %d" % i, "body": "body %d" % i, "priority": "low"})
        check(st == 201, "ticket %d -> %s %r" % (i, st, t))
        tickets[t["id"]] = ("ticket %d" % i, t["owner"])
    return tokens, tickets


def verify(s, tokens, tickets, label):
    check(sorted(list_all(s)) == sorted(tickets), "%s: ticket set differs after restart" % label)
    for tid, (title, owner) in list(tickets.items())[::7]:
        st, t = s.req("GET", "/v1/tickets/" + tid, ADMIN)
        check(st == 200 and t["title"] == title and t["owner"] == owner, "%s: %s changed" % (label, tid))
    for secret, (tok_id, role) in tokens.items():
        st, me = s.req("GET", "/v1/me", secret)
        check(st == 200 and me["id"] == tok_id and me["role"] == role, "%s: token %s not valid after restart" % (label, tok_id))
    st, toks = s.req("GET", "/v1/tokens", ADMIN)
    check(st == 200 and len(toks["items"]) == len(tokens), "%s: token list size" % label)


def verify_authorization(s, tokens, tickets):
    viewer = next(sec for sec, (_, r) in tokens.items() if r == "viewer")
    members = [sec for sec, (_, r) in tokens.items() if r == "member"]
    tid, (_, owner) = next(iter(tickets.items()))
    other = next(m for m in members if tokens[m][0] != owner)
    st, _ = s.req("POST", "/v1/tickets", viewer, {"title": "x"})
    check(st == 403, "viewer create after restart -> %s" % st)
    st, _ = s.req("PATCH", "/v1/tickets/" + tid, other, {"title": "hijack"})
    check(st == 403, "non-owner member patch after restart -> %s" % st)
    st, _ = s.req("GET", "/v1/me", ADMIN + "x")
    check(st == 401, "wrong bootstrap token after restart -> %s" % st)


def scenario_restart_with_realistic_state():
    work = tempfile.mkdtemp(prefix="td-restart-")
    try:
        s = Srv(work).wait()
        tokens, tickets = populate(s)
        s.stop()
        started = time.time()
        s = Srv(work).wait()  # the regression: this start used to exit 1 with LIMIT_EXCEEDED
        print("  restart with %d tickets and %d tokens took %.1fs" % (len(tickets), len(tokens), time.time() - started))
        verify(s, tokens, tickets, "after SIGTERM restart")
        verify_authorization(s, tokens, tickets)
        s.stop()
        # A second restart from the same file must also work, and writes keep landing.
        s = Srv(work).wait()
        st, t = s.req("POST", "/v1/tickets", ADMIN, {"title": "after restart"})
        check(st == 201, "create after restart -> %s" % st)
        tickets[t["id"]] = ("after restart", "bootstrap")
        s.stop()
        s = Srv(work).wait()
        verify(s, tokens, tickets, "second restart")
        s.stop()
    finally:
        shutil.rmtree(work, ignore_errors=True)


def scenario_invalid_state_rejected_without_damage():
    work = tempfile.mkdtemp(prefix="td-restart-bad-")
    try:
        s = Srv(work).wait()
        populate_small = [s.req("POST", "/v1/tickets", ADMIN, {"title": "t%d" % i})[0] for i in range(20)]
        check(all(c == 201 for c in populate_small), "seed tickets")
        s.stop()
        path = os.path.join(work, "state.json")
        good = open(path, "rb").read()
        for label, damaged in (("truncated", good[: len(good) // 2]), ("not an object", b"[]"),
                               ("bad version", good.replace(b'"version": 2', b'"version": 9', 1))):
            with open(path, "wb") as f:
                f.write(damaged)
            bad = Srv(work)
            try:
                bad.p.wait(15)
            except subprocess.TimeoutExpired:
                bad.p.kill()
                raise AssertionError("%s state: server did not exit" % label)
            err = bad.p.stderr.read().decode()
            check(bad.p.returncode == 65, "%s state: exit %s, stderr %r" % (label, bad.p.returncode, err))
            check(err.startswith("ticketdesk: ") and '"phase"' not in err, "%s state: stderr shape %r" % (label, err))
            check(open(path, "rb").read() == damaged, "%s state: file was modified by a failed start" % label)
    finally:
        shutil.rmtree(work, ignore_errors=True)


def scenario_sigkill_keeps_acknowledged_writes():
    work = tempfile.mkdtemp(prefix="td-restart-kill-")
    try:
        s = Srv(work).wait()
        tokens, tickets = populate_base(s, 400, 40)
        acked = dict(tickets)
        for rnd in range(4):
            def create(i, srv=s, r=rnd):
                try:
                    st, t = srv.req("POST", "/v1/tickets", ADMIN, {"title": "k%d-%d" % (r, i)}, timeout=5)
                    return (t["id"], "k%d-%d" % (r, i)) if st == 201 else None
                except Exception:
                    return None

            with cf.ThreadPoolExecutor(8) as ex:
                futures = [ex.submit(create, i) for i in range(60)]
                time.sleep(0.4)
                s.p.send_signal(signal.SIGKILL)
                s.p.wait()
                for f in futures:
                    got = f.result()
                    if got:
                        acked[got[0]] = (got[1], "bootstrap")
            s = Srv(work).wait()
            ids = set(list_all(s))
            missing = [t for t in acked if t not in ids]
            check(not missing, "round %d: %d acknowledged tickets lost after SIGKILL, e.g. %s" % (rnd, len(missing), missing[:3]))
            check(len(ids) == len(list_all(s)), "round %d: duplicate ids in listing" % rnd)
        s.stop()
    finally:
        shutil.rmtree(work, ignore_errors=True)


def populate_base(s, n_tickets, n_tokens):
    tokens = {}
    for i in range(n_tokens):
        st, t = s.req("POST", "/v1/tokens", ADMIN, {"name": "t%d" % i, "role": "viewer"})
        check(st == 201, "token %d" % i)
        tokens[t["token"]] = (t["id"], "viewer")
    tickets = {}
    for i in range(n_tickets):
        st, t = s.req("POST", "/v1/tickets", ADMIN, {"title": "base %d" % i})
        check(st == 201, "ticket %d" % i)
        tickets[t["id"]] = ("base %d" % i, "bootstrap")
    return tokens, tickets


TESTS = [
    ("restart_with_1000_tickets_100_tokens_default_ceilings", scenario_restart_with_realistic_state),
    ("invalid_state_rejected_without_damage", scenario_invalid_state_rejected_without_damage),
    ("sigkill_keeps_acknowledged_writes_at_default_ceilings", scenario_sigkill_keeps_acknowledged_writes),
]

if __name__ == "__main__":
    failed = 0
    for name, fn in TESTS:
        t0 = time.time()
        try:
            fn()
            print("PASS %s (%.0fs)" % (name, time.time() - t0))
        except AssertionError as e:
            failed += 1
            print("FAIL %s: %s" % (name, e))
    print("%d tests, %d failed" % (len(TESTS), failed))
    sys.exit(1 if failed else 0)
