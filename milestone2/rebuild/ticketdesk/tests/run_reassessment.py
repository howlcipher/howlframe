#!/usr/bin/env python3
"""Independent fresh-candidate M2 probes. Python stdlib only.

HF_BIN=... APP_ARTIFACT=... python3 run_reassessment.py LAUNCHER -v
Reuses retained transport/startup assertions; no legacy-state recovery
assumptions.
"""

import concurrent.futures
import hashlib
import http.client
import json
from pathlib import Path
import signal
import socket
import subprocess
import sys
import time
from types import SimpleNamespace

BASELINE = (
    Path(__file__).resolve().parents[3] / "ticketdesk/tests/reassessment"
)
sys.path.insert(0, str(BASELINE))
import run_reassessment as baseline  # noqa: E402 (shared retained probes)
from harness import (  # noqa: E402 (local harness path set above)
    TESTS,
    check,
    main,
    test,
)  # noqa: E402 (shared process harness)

TESTS[:] = [
    entry
    for entry in TESTS
    if entry[0]
    in {
        "fresh.startup.busy_unchanged",
        "fresh.startup.network_unchanged",
        "fresh.upload.truncated",
    }
]


@test(
    "candidate.state_shape",
    ["R1.3"],
    "invalid v2 buckets and ticket shapes rejected unchanged",
)
def invalid_state(ctx):
    valid = {
        "id": "t1",
        "title": "valid",
        "body": "",
        "priority": "normal",
        "status": "open",
        "owner": "bootstrap",
        "created_at": 1,
        "updated_at": 1,
    }
    shapes = [
        {"bucket:000000000000": {"order": ["t1"], "records": {}}},
        {"bucket:000000000000": {"order": "invalid", "records": {}}},
        {
            "bucket:000000000000": {
                "order": ["t1"],
                "records": {"t1": {"id": "t1"}},
            }
        },
        {"token_counter": {"next": "not-an-integer"}},
        {"bucket:000000000001": {"order": ["t1"], "records": {"t1": valid}}},
        {
            "bucket:000000000000": {
                "order": ["t1"],
                "records": {"t1": {**valid, "title": 1}},
            }
        },
    ]
    for extra in shapes:
        srv = ctx.make()
        baseline.write_state(
            srv, {"header": {"version": 2, "next": 2}, **extra}
        )
        before = Path(srv.data).read_bytes()
        srv.spawn()
        check(srv.wait_exit(10) == 65, "invalid v2 state accepted", extra)
        check(
            Path(srv.data).read_bytes() == before,
            "rejected state changed",
            extra,
        )
        check(
            srv.stderr().startswith("ticketdesk: "),
            "unsafe error",
            srv.stderr(),
        )


def interrupted(srv, method, path, token=None, headers=None):
    connection = socket.create_connection(("127.0.0.1", srv.port), timeout=5)
    text = (
        "%s %s HTTP/1.1\r\nHost: localhost\r\nContent-Length: "
        "100\r\nConnection: close\r\n" % (method, path)
    )
    if token:
        text += "Authorization: Bearer " + token + "\r\n"
    for name, value in headers or []:
        text += name + ": " + value + "\r\n"
    connection.sendall((text + "\r\n{}").encode("ascii"))
    connection.shutdown(socket.SHUT_WR)
    response = http.client.HTTPResponse(connection)
    response.begin()
    result = response.status, json.loads(response.read())
    connection.close()
    return result


@test(
    "candidate.truncated_order",
    ["R3.4", "R15"],
    "route/method/auth/fault checks beat truncated body",
)
def truncated_order(ctx):
    with ctx.fresh(env={"TICKETDESK_ALLOW_DEBUG_FAULT": "1"}) as srv:
        cases = [
            ("POST", "/missing", None, [], 404, "not_found"),
            ("POST", "/v1/me", None, [], 405, "method_not_allowed"),
            ("POST", "/v1/tickets", None, [], 401, "unauthorized"),
            (
                "POST",
                "/v1/tickets",
                srv.admin,
                [],
                415,
                "unsupported_media_type",
            ),
            (
                "POST",
                "/v1/tickets",
                None,
                [("X-Debug-Fault", "1")],
                500,
                "internal_error",
            ),
        ]
        for method, path, token, headers, status, code in cases:
            got, body = interrupted(srv, method, path, token, headers)
            check(
                got == status and body["error"]["code"] == code,
                "earlier check lost to transport error: " + path,
                (got, body),
            )


@test(
    "candidate.ignored_body",
    ["R3.4"],
    "GET and DELETE ignore a stalled declared body immediately",
)
def ignored_body(ctx):
    with ctx.fresh() as srv:
        created = srv.req(
            "POST", "/v1/tickets", srv.admin, {"title": "delete with body"}
        )
        check(created.status == 201, "setup create failed", created)
        for method, path in (
            ("GET", "/v1/health"),
            ("DELETE", "/v1/tickets/" + created.json["id"]),
        ):
            connection = socket.create_connection(
                ("127.0.0.1", srv.port), timeout=2
            )
            request = (
                "%s %s HTTP/1.1\r\nHost: localhost\r\nAuthorization: "
                "Bearer %s\r\n"
                "Content-Length: 100\r\nConnection: close\r\n\r\nx"
                % (method, path, srv.admin)
            )
            connection.sendall(request.encode("ascii"))
            # No half-close: the upload remains stalled while we
            # wait for response.
            response = http.client.HTTPResponse(connection)
            response.begin()
            check(
                response.status == 200,
                "ignored body changed response",
                response.status,
            )
            check(
                isinstance(json.loads(response.read()), dict),
                "missing JSON body",
            )
            connection.close()


@test(
    "candidate.default_limits",
    ["R5", "R6", "R14"],
    "default ceilings handle boundary records across buckets",
)
def default_limits(ctx):
    srv = ctx.make()
    # The frozen harness raises VM budgets. This probe deliberately uses
    # defaults.
    original = srv.environment

    def environment():
        env = original()
        env["HF_RUN_FLAGS"] = ""
        return env

    srv.environment = environment
    srv.start()
    for number in range(150):
        response = srv.req(
            "POST",
            "/v1/tickets",
            srv.admin,
            {"title": "é" * 120, "body": "界" * 4000},
        )
        check(
            response.status == 201,
            "valid boundary record failed at %d" % number,
            response,
        )
    response = srv.req("GET", "/v1/tickets?limit=100", srv.admin)
    check(
        response.status == 200 and len(response.json["items"]) == 100,
        "default-budget page failed",
        response,
    )
    srv.restart(sig=signal.SIGKILL)
    response = srv.req(
        "GET",
        "/v1/tickets?limit=100&cursor=" + response.json["next_cursor"],
        srv.admin,
    )
    check(
        response.status == 200 and len(response.json["items"]) == 50,
        "default-budget page after restart failed",
        response,
    )


def crash_request(srv, method, path, token, body):
    """Do not swallow BrokenPipe then construct HTTPResponse(None).

    The shared frozen harness tolerates a broken upload before getresponse;
    Python 3.14 can then raise AttributeError while constructing HTTPResponse.
    This transport leaves that OSError visible to the crash worker instead.
    """
    connection = http.client.HTTPConnection("127.0.0.1", srv.port, timeout=3)
    try:
        headers = {"Authorization": "Bearer " + token, "Connection": "close"}
        payload = None
        if body is not None:
            payload = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        connection.request(method, path, payload, headers)
        with connection.getresponse() as response:
            return SimpleNamespace(
                status=response.status, json=json.loads(response.read())
            )
    finally:
        connection.close()


@test(
    "candidate.kill_consistency",
    ["R13"],
    "SIGKILL concurrent creates keep acknowledgments and list/GET consistent",
)
def kill_consistency(ctx):
    srv = ctx.make()
    srv.start()
    confirmed = {}
    for cycle in range(6):

        def create(number):
            try:
                response = crash_request(
                    srv,
                    "POST",
                    "/v1/tickets",
                    srv.admin,
                    {"title": "cycle %d request %d" % (cycle, number)},
                )
                return response.json if response.status == 201 else None
            except (OSError, http.client.HTTPException):
                return None

        with concurrent.futures.ThreadPoolExecutor(max_workers=8) as workers:
            futures = [workers.submit(create, number) for number in range(70)]
            check(
                futures[0].result() is not None,
                "no successful create before kill",
            )
            time.sleep(0.025 + cycle * 0.009)
            srv.stop(signal.SIGKILL)
            for future in futures:
                record = future.result()
                if record:
                    confirmed[record["id"]] = record
        srv.start()
        seen = {}
        cursor = None
        while True:
            path = "/v1/tickets?limit=100" + (
                "&cursor=" + cursor if cursor else ""
            )
            response = srv.req("GET", path, srv.admin)
            check(response.status == 200, "list after kill failed", response)
            for ticket in response.json["items"]:
                check(ticket["id"] not in seen, "duplicate listed id", ticket)
                seen[ticket["id"]] = ticket
            cursor = response.json["next_cursor"]
            if cursor is None:
                break
        check(
            confirmed.items() <= seen.items(),
            "acknowledged create lost across kill",
        )
        for ticket in seen.values():
            response = srv.req("GET", "/v1/tickets/" + ticket["id"], srv.admin)
            check(
                response.status == 200 and response.json == ticket,
                "list/GET mismatch",
                response,
            )


@test(
    "candidate.kill_updates",
    ["R13"],
    "SIGKILL patch/delete preserves acknowledgments and bucket consistency",
)
def kill_updates(ctx):
    srv = ctx.make()
    srv.start()
    tickets = []
    for number in range(130):
        response = srv.req(
            "POST", "/v1/tickets", srv.admin, {"title": "before %d" % number}
        )
        check(response.status == 201, "setup failed", response)
        tickets.append(response.json["id"])

    def mutate(pair):
        number, ticket_id = pair
        method = "DELETE" if number % 3 == 0 else "PATCH"
        try:
            response = crash_request(
                srv,
                method,
                "/v1/tickets/" + ticket_id,
                srv.admin,
                {"title": "after %d" % number} if method == "PATCH" else None,
            )
            return (
                (method, ticket_id, response.json)
                if response.status == 200
                else None
            )
        except (OSError, http.client.HTTPException):
            return None

    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as workers:
        futures = [workers.submit(mutate, pair) for pair in enumerate(tickets)]
        check(
            futures[0].result() is not None,
            "no successful mutation before kill",
        )
        time.sleep(0.025)
        srv.stop(signal.SIGKILL)
        acknowledged = [
            value for future in futures if (value := future.result())
        ]
    check(
        acknowledged,
        "no mutations acknowledged before kill; test did not "
        "exercise durability",
    )
    srv.start()
    seen = {}
    cursor = None
    while True:
        response = srv.req(
            "GET",
            "/v1/tickets?limit=100" + ("&cursor=" + cursor if cursor else ""),
            srv.admin,
        )
        check(
            response.status == 200, "list after mutation kill failed", response
        )
        for record in response.json["items"]:
            check(
                record["id"] not in seen,
                "duplicate after mutation kill",
                record,
            )
            seen[record["id"]] = record
        cursor = response.json["next_cursor"]
        if cursor is None:
            break
    for method, ticket_id, record in acknowledged:
        response = srv.req("GET", "/v1/tickets/" + ticket_id, srv.admin)
        if method == "DELETE":
            check(
                response.status == 404 and ticket_id not in seen,
                "acknowledged delete lost",
                response,
            )
        else:
            check(
                response.status == 200
                and response.json == record
                and seen.get(ticket_id) == record,
                "acknowledged patch lost or unlisted",
                response,
            )
    for ticket_id in tickets:
        response = srv.req("GET", "/v1/tickets/" + ticket_id, srv.admin)
        check(
            (response.status == 404 and ticket_id not in seen)
            or (
                response.status == 200 and seen.get(ticket_id) == response.json
            ),
            "list/GET disagreement after mutation kill",
            response,
        )


@test(
    "candidate.token_interruption",
    ["R13"],
    "atomic token publication preserves confirmed state across real kills",
)
def token_interruption(ctx):
    # Root adapted this independent scenario after replacing the intermediate
    # three-write token design. Real SIGKILLs now exercise the single-record
    # publication; removing a nonexistent digest index would be a vacuous test.
    srv = ctx.make()
    srv.start()
    stable = srv.req(
        "POST",
        "/v1/tokens",
        srv.admin,
        {"name": "confirmed", "role": "member"},
    )
    check(stable.status == 201, "stable token setup failed", stable)
    confirmed = [stable.json["token"]]
    ticket = srv.req(
        "POST", "/v1/tickets", confirmed[0], {"title": "confirmed ticket"}
    )
    check(ticket.status == 201, "stable ticket setup failed", ticket)
    for cycle in range(3):

        def create(number):
            try:
                result = crash_request(
                    srv,
                    "POST",
                    "/v1/tokens",
                    srv.admin,
                    {"name": "token %d" % number, "role": "viewer"},
                )
                return result.json["token"] if result.status == 201 else None
            except (OSError, http.client.HTTPException):
                return None

        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as workers:
            futures = [workers.submit(create, number) for number in range(40)]
            check(
                futures[0].result() is not None, "no token issued before kill"
            )
            time.sleep(0.008 + cycle * 0.004)
            srv.stop(signal.SIGKILL)
            confirmed.extend(
                secret for future in futures if (secret := future.result())
            )
        state = baseline.load_state(srv)
        check(
            not any(key.startswith(("token:", "digest:")) for key in state),
            "a split token index reappeared",
            state.keys(),
        )
        group = state["tokens"]
        check(
            set(group["order"]) == set(group["records"]),
            "token publication split record/order",
        )
        srv.start()
        for secret in confirmed:
            check(
                srv.req("GET", "/v1/me", secret).status == 200,
                "acknowledged token lost after crash",
            )
        response = srv.req(
            "GET", "/v1/tickets/" + ticket.json["id"], confirmed[0]
        )
        check(
            response.status == 200 and response.json == ticket.json,
            "prior ticket lost after interrupted token publication",
            response,
        )


@test(
    "candidate.migration",
    ["R13"],
    "offline migration preserves orphan legacy records, tokens and counters",
)
def migration(ctx):
    source = Path(ctx.tmp()) / "legacy.json"
    srv = ctx.make()
    state = {"meta": {"next_ticket": 100, "next_token": 10}}
    expected = []
    for number in (1, 64, 65):
        record = {
            "id": "t%d" % number,
            "title": "ticket %d" % number,
            "body": "世界",
            "priority": "high",
            "status": "done",
            "owner": "bootstrap",
            "created_at": 1760000000,
            "updated_at": 1760000001,
        }
        state["t:%010d" % number] = record
        expected.append(record)
    secrets = []
    # Deliberately omit old b:/q:/h: indexes to verify canonical-record
    # migration.
    for number, role in enumerate(("viewer", "member", "admin"), start=1):
        token_id = "tok_" + ("%016x" % (4 - number))
        secret = "td_" + ("%064x" % number)
        secrets.append(secret)
        state["k:" + token_id] = {
            "id": token_id,
            "name": "role " + role,
            "role": role,
            "created_at": 1760000000,
            "revoked": int(role == "viewer"),
            "hash": hashlib.sha256(secret.encode()).hexdigest(),
            "seq": number,
        }
    source.write_text(json.dumps(state), encoding="utf-8")
    before = source.read_bytes()
    command = [
        sys.executable,
        str(Path(__file__).resolve().parents[1] / "migrate_state.py"),
        str(source),
        srv.data,
    ]
    result = subprocess.run(command, capture_output=True, timeout=10)
    check(result.returncode == 0, "migration failed", result.stderr.decode())
    check(source.read_bytes() == before, "migration changed source")
    migrated = Path(srv.data).read_bytes()
    check(
        not any(secret.encode() in migrated for secret in secrets),
        "migration leaked secrets",
    )
    result = subprocess.run(command, capture_output=True, timeout=10)
    check(
        result.returncode != 0 and Path(srv.data).read_bytes() == migrated,
        "migration overwrote existing destination",
    )
    srv.start()
    response = srv.req("GET", "/v1/tickets?limit=100", srv.admin)
    check(
        response.status == 200 and response.json["items"] == expected,
        "migration lost/changed orphan records",
        response,
    )
    response = srv.req("POST", "/v1/tickets", secrets[1], {"title": "member"})
    check(
        response.status == 201 and response.json["id"] == "t100",
        "migration lost counter/member",
        response,
    )
    response = srv.req("GET", "/v1/me", secrets[0])
    check(response.status == 401, "migration lost revocation", response)
    response = srv.req("GET", "/v1/tokens", secrets[2])
    check(
        response.status == 200
        and [item["role"] for item in response.json["items"]]
        == ["viewer", "member", "admin"],
        "migration lost token order/admin",
        response,
    )
    srv.restart(sig=signal.SIGKILL)
    check(
        srv.req("GET", "/v1/me", secrets[1]).status == 200,
        "migrated token lost on restart",
    )


if __name__ == "__main__":
    sys.exit(main())
