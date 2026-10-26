#!/usr/bin/env python3
"""Retained independent M2 reassessment probes, 2026-10-20.

Usage: HF_BIN=... APP_ARTIFACT=... python3 run_reassessment.py LAUNCHER -v
Frozen requirements and acceptance tests are unchanged. Imports only the
process/
HTTP harness, and asserts independent expectations here. Recovery probes are
supplemental regression coverage of R13 consistency, not new milestone gates.
"""

import http.client
import json
import re
from pathlib import Path
import signal
import socket
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "acceptance"))
from harness import check, main, test  # noqa: E402 (local harness path)


def write_state(server, state):
    with open(server.data, "w", encoding="utf-8") as stream:
        json.dump(state, stream)


def load_state(server):
    with open(server.data, encoding="utf-8") as stream:
        return json.load(stream)


@test(
    "fresh.startup.busy_unchanged",
    ["R1.3"],
    "occupied port preserves existing empty state bytes",
)
def busy_unchanged(ctx):
    busy = socket.socket()
    try:
        busy.bind(("0.0.0.0", 0))
        busy.listen()
        srv = ctx.make(port=busy.getsockname()[1])
        Path(srv.data).write_bytes(b"")
        srv.spawn()
        check(srv.wait_exit(10) not in (None, 0), "occupied port must fail")
        check(
            Path(srv.data).read_bytes() == b"",
            "failed startup changed existing zero-byte state",
        )
    finally:
        busy.close()


@test(
    "fresh.startup.network_unchanged",
    ["R1.3", "R2.2"],
    "missing network preserves existing state bytes",
)
def network_unchanged(ctx):
    srv = ctx.make()
    srv.grants = srv.exact_grants(srv.dir, omit="network")
    original = b"{}\n"
    Path(srv.data).write_bytes(original)
    srv.spawn()
    check(
        srv.wait_exit(10) == 77, "missing network must exit 77", srv.stderr()
    )
    check(
        Path(srv.data).read_bytes() == original,
        "failed startup rewrote existing state",
    )


@test(
    "fresh.startup.state_shape",
    ["R1.3"],
    "invalid counters and indexed record shapes fail safely",
)
def state_shape(ctx):
    cases = [
        {"meta": {"next_ticket": 0, "next_token": 1}},
        {"meta": {"next_ticket": 1, "next_token": -1}},
        {
            "meta": {"next_ticket": 2, "next_token": 1},
            "t:0000000001": {"id": "t1"},
        },
    ]
    for state in cases:
        srv = ctx.make()
        write_state(srv, state)
        before = Path(srv.data).read_bytes()
        srv.spawn()
        check(srv.wait_exit(10) == 65, "invalid state should exit 65", state)
        check(
            Path(srv.data).read_bytes() == before,
            "invalid state was modified",
            state,
        )
        check(
            srv.stderr().startswith("ticketdesk: "),
            "unsafe startup error",
            srv.stderr(),
        )


def recovery_case(ctx, mode):
    srv = ctx.make()
    srv.start()
    expected = []
    for number in range(1, 68):
        response = srv.req(
            "POST", "/v1/tickets", srv.admin, {"title": "ticket %d" % number}
        )
        check(response.status == 201, "create failed", response)
        expected.append(response.json)
    srv.stop(signal.SIGKILL)
    state = load_state(srv)
    if mode == "legacy":
        state = {
            key: value
            for key, value in state.items()
            if not key.startswith("b:")
        }
    elif mode == "unindexed":
        for key, value in state.items():
            if key.startswith("b:"):
                value["ids"] = [
                    number
                    for number in value["ids"]
                    if number not in (1, 64, 65)
                ]
    elif mode == "dangling":
        del state["t:0000000002"]
        expected = [ticket for ticket in expected if ticket["id"] != "t2"]
    write_state(srv, state)
    srv.start()
    seen = []
    cursor = None
    while True:
        path = "/v1/tickets?limit=7" + ("&cursor=" + cursor if cursor else "")
        response = srv.req("GET", path, srv.admin, timeout=5)
        check(response.status == 200, "recovery list failed", response)
        seen.extend(response.json["items"])
        cursor = response.json["next_cursor"]
        if cursor is None:
            break
        check(
            len(seen) <= len(expected), "pagination loop or duplicate tickets"
        )
    check(
        seen == expected,
        "listing lost/changed confirmed records after " + mode,
    )
    for ticket in expected:
        response = srv.req("GET", "/v1/tickets/" + ticket["id"], srv.admin)
        check(
            response.status == 200 and response.json == ticket,
            "GET differs after recovery",
            response,
        )
    response = srv.req(
        "POST", "/v1/tickets", srv.admin, {"title": "after recovery"}
    )
    check(
        response.status == 201 and response.json["id"] == "t68",
        "counter lost during recovery",
        response,
    )
    srv.restart(sig=signal.SIGKILL)
    response = srv.req("GET", "/v1/tickets?limit=100", srv.admin)
    check(
        response.status == 200
        and len(response.json["items"]) == len(expected) + 1,
        "recovery did not survive a second restart",
        response,
    )


@test(
    "fresh.recovery.legacy",
    ["R13"],
    "supplemental: pre-index records retain listing on migration",
)
def legacy(ctx):
    recovery_case(ctx, "legacy")


@test(
    "fresh.recovery.unindexed",
    ["R13"],
    "supplemental: bucket-edge unindexed confirmed records recover",
)
def unindexed(ctx):
    recovery_case(ctx, "unindexed")


@test(
    "fresh.recovery.dangling",
    ["R13"],
    "supplemental: dangling index removes only the missing record",
)
def dangling(ctx):
    recovery_case(ctx, "dangling")


@test(
    "fresh.upload.truncated",
    ["R3.3", "R7.1", "R8", "R16"],
    "truncated upload keeps error schema, request id and access log",
)
def truncated(ctx):
    with ctx.fresh(flags="--access-log") as srv:
        connection = socket.create_connection(
            ("127.0.0.1", srv.port), timeout=5
        )
        payload = (
            "POST /v1/tickets HTTP/1.1\r\nHost: localhost\r\n"
            "Authorization: Bearer %s\r\nContent-Type: application/json\r\n"
            "Content-Length: 100\r\nConnection: close\r\n\r\n"
            '{"title":"complete JSON but short transport"}' % srv.admin
        )
        connection.sendall(payload.encode("ascii"))
        connection.shutdown(socket.SHUT_WR)
        response = http.client.HTTPResponse(connection)
        response.begin()
        body = json.loads(response.read())
        request_id = response.getheader("X-Request-Id")
        check(response.status == 400, "truncated body must fail")
        check(
            re.fullmatch(r"[0-9a-f]{32}", request_id or ""),
            "missing request id",
            body,
        )
        check(
            set(body) == {"error"} and isinstance(body["error"], dict),
            "wrong error shape",
            body,
        )
        check(
            set(body["error"]) == {"code", "message", "request_id"},
            "wrong error keys",
            body,
        )
        check(
            body["error"]["code"] == "invalid_json"
            and body["error"]["request_id"] == request_id,
            "wrong truncated body code/id",
            body,
        )
        connection.close()
        # An ordinary round trip ensures the request has completed
        # before reading logs.
        check(
            srv.req("GET", "/v1/tickets", srv.admin).json["items"] == [],
            "truncated upload produced a mutation",
        )
        lines = [
            line
            for line in srv.stderr().splitlines()
            if "request_id=" + request_id in line
        ]
        check(
            len(lines) == 1
            and re.fullmatch(
                "access method=POST path=/v1/tickets status=400 "
                "duration_ms=[0-9]+ request_id=" + request_id,
                lines[0],
            ),
            "missing or malformed truncated-upload access line",
            lines,
        )


if __name__ == "__main__":
    sys.exit(main())
