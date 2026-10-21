"""Executable implementation evidence, additional to frozen independent tests.

Only committed sanitized fixtures and temporary stores are used. No live
sibling
telemetry is accessed. Requires Python Playwright and locally installed
Chromium.
"""

from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

APP = Path(__file__).resolve().parents[1]
REPO = APP.parents[1]
BIN = Path(os.environ.get("HOWLFRAME_BIN", APP / "build/howlframe")).resolve()
FIXTURE = APP / "tests/fixtures/review.jsonl"
BOARD = Path(
    os.environ.get("HOWLBOARD_REPO", REPO.parent / "howlboard")
).resolve()
ENV = dict(os.environ, HOWLFRAME_BIN=str(BIN), NO_PROXY="127.0.0.1,localhost")


def command(argv, **kwargs):
    result = subprocess.run(
        list(map(str, argv)),
        env=ENV,
        text=True,
        capture_output=True,
        timeout=120,
        **kwargs,
    )
    assert result.returncode == 0, (
        argv,
        result.returncode,
        result.stdout,
        result.stderr,
    )
    return result.stdout


def free_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


def request(base, method, path, body=None, media="application/json"):
    if body is not None and not isinstance(body, bytes):
        body = json.dumps(body, ensure_ascii=False).encode()
    req = urllib.request.Request(
        base + path, body, method=method, headers={"Content-Type": media}
    )
    try:
        response = urllib.request.urlopen(req, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.status, json.loads(response.read())


@contextmanager
def running(argv, cwd, env, ready):
    with tempfile.TemporaryFile(mode="w+") as log:
        process = subprocess.Popen(
            list(map(str, argv)),
            cwd=cwd,
            env=env,
            stdout=log,
            stderr=log,
            text=True,
        )
        try:
            deadline = time.monotonic() + 10
            while True:
                if process.poll() is not None:
                    log.seek(0)
                    raise AssertionError(("server exited", log.read()))
                try:
                    if ready():
                        break
                except (OSError, ValueError):
                    pass
                assert time.monotonic() < deadline, "server readiness timeout"
                time.sleep(0.05)
            yield process
        finally:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
            log.seek(0)
            output = log.read()
            if "fault request_id=" in output:
                print("SERVER DIAGNOSTICS:", output, flush=True)
            assert "panic:" not in output, output


@contextmanager
def desk(data):
    port = free_port()
    base = f"http://127.0.0.1:{port}"
    env = dict(ENV, EVIDENCEDESK_PORT=str(port), EVIDENCEDESK_DATA=str(data))
    with running(
        [APP / "scripts/run.sh"],
        APP,
        env,
        lambda: request(base, "GET", "/api/health")[0] == 200,
    ):
        yield base, port


def api_and_restart(root):
    data = root / "state.json"
    records = [json.loads(line) for line in FIXTURE.read_text().splitlines()]
    with desk(data) as (base, port):
        env = dict(
            ENV,
            EVIDENCEDESK_URL=base,
            PROJECTOR_ARGV=json.dumps([str(APP / "scripts/project.sh")]),
        )
        result = subprocess.run(
            [
                "python3",
                "-m",
                "unittest",
                "discover",
                "-s",
                str(APP / "tests"),
                "-p",
                "acceptance.py",
                "-v",
            ],
            env=env,
            capture_output=True,
            text=True,
            timeout=120,
        )
        print(result.stdout + result.stderr, end="")
        assert result.returncode == 0
        assert (
            request(
                base,
                "POST",
                "/api/import",
                FIXTURE.read_bytes(),
                "application/x-ndjson",
            )[0]
            == 200
        )
        # Both application and runner pre-read ceilings must yield the same
        # 413 class; the latter previously escaped as a generic 500.
        for size in (262145, 300001, 400000):
            code, failure = request(
                base,
                "POST",
                "/api/import",
                b"x" * size,
                "application/x-ndjson",
            )
            assert (
                code == 413 and failure["error"]["code"] == "payload_too_large"
            )
            assert request(base, "GET", "/api/tasks")[1]["items"]
        note = "Persistent reviewer annotation café 😀"
        assert (
            request(base, "PATCH", "/api/tasks/REVIEW-1/note", {"note": note})[
                0
            ]
            == 200
        )
        # Live socket ownership, not merely a script argument assertion.
        listeners = command(["ss", "-ltnp"])
        matching = [
            line for line in listeners.splitlines() if f":{port} " in line
        ]
        assert matching and all(
            f"127.0.0.1:{port}" in line for line in matching
        ), matching
        print("PASS live loopback listener:", matching[0].strip())
    with desk(data) as (base, _):
        detail = request(base, "GET", "/api/tasks/REVIEW-1")[1]
        assert detail["events"] == records[:2] and detail["note"] == note
        # URL encoding, astral Unicode bounds, unknown actions, exact
        # retention.
        unusual = dict(
            records[0],
            entry_id="encoded",
            task_id="café space?#%😀",
            action="unknown_action",
            result={"nested": [1, False]},
        )
        raw = json.dumps(unusual, ensure_ascii=False).encode()
        assert (
            request(base, "POST", "/api/import", raw, "application/x-ndjson")[
                0
            ]
            == 200
        )
        path = "/api/tasks/" + urllib.parse.quote(unusual["task_id"], safe="")
        assert request(base, "GET", path)[1]["events"] == [unusual]
        for value, expected in [("😀" * 2000, 200), ("😀" * 2001, 400)]:
            assert (
                request(base, "PATCH", path + "/note", {"note": value})[0]
                == expected
            )
        assert (
            request(base, "PATCH", path + "/note", {"note": ""})[1]["note"]
            == ""
        )
        assert (
            request(
                base,
                "POST",
                "/api/import",
                b"\n \n{broken",
                "application/x-ndjson",
            )[1]["error"]["line"]
            == 3
        )
    print(
        "PASS exact restart persistence, encoded ID, Unicode "
        "notes, physical line diagnostics"
    )


def browser(root):
    from playwright.sync_api import sync_playwright, expect

    with desk(root / "browser.json") as (
        base,
        _,
    ), sync_playwright() as playwright:
        launch = {"headless": True}
        if os.environ.get("CHROMIUM_EXECUTABLE"):
            launch["executable_path"] = os.environ["CHROMIUM_EXECUTABLE"]
        browser = playwright.chromium.launch(**launch)
        page = browser.new_page()
        page.set_default_timeout(10000)
        errors = []
        page.on("pageerror", lambda error: errors.append(str(error)))
        page.goto(base)
        expect(page.locator("#status")).to_contain_text("Ready")
        assert page.get_by_label("Ledger JSONL", exact=False).count() == 1
        assert page.get_by_label("Filter task IDs").count() == 1
        page.fill("#ledger-input", FIXTURE.read_text())
        with page.expect_request("**/api/import") as upload:
            page.focus("#import-button")
            page.keyboard.press("Enter")
        assert upload.value.headers["content-type"] == "application/x-ndjson"
        expect(page.locator("#status")).to_contain_text(
            "Imported 3 entries in 2 tasks"
        )
        expect(page.locator("#task-list option")).to_have_count(2)
        page.fill("#task-filter", "REVIEW-1")
        expect(page.locator("#task-list option")).to_have_count(1)
        page.focus("#task-list")
        page.keyboard.press("ArrowDown")
        # Selection via native keyboard may leave the already highlighted first
        # option unchanged; press Home followed by Enter, then dispatch through
        # native select_option if no change event was generated by this engine.
        page.keyboard.press("Home")
        page.keyboard.press("Enter")
        page.select_option("#task-list", "0")
        expect(page.locator("#task-detail")).to_contain_text("claimed")
        events = page.locator("#task-detail li").all_text_contents()
        assert (
            len(events) == 2
            and "task_created" in events[0]
            and "verification_completed" in events[1]
        )
        assert "passed" not in page.locator("#task-detail").inner_text()
        note = "Browser review <b>literal</b> café 😀"
        page.fill("#note-input", note)
        page.focus("#save-note")
        page.keyboard.press("Space")
        expect(page.locator("#status")).to_contain_text("Annotation saved")
        page.reload()
        expect(page.locator("#task-list option")).to_have_count(2)
        page.select_option("#task-list", "0")
        expect(page.locator("#note-input")).to_have_value(note)
        page.fill("#task-filter", "REVIEW-2")
        page.select_option("#task-list", "1")
        expect(page.locator("#task-detail")).to_contain_text("task_completed")
        page.fill("#task-filter", "")
        expect(page.locator("#task-list option")).to_have_count(2)
        page.fill(
            "#ledger-input", FIXTURE.read_text().splitlines()[0] + "\n{broken"
        )
        page.click("#import-button")
        expect(page.locator("#status")).to_contain_text("line 2")
        expect(page.locator("#task-list option")).to_have_count(2)
        expect(page.locator("#task-detail")).to_contain_text("REVIEW-2")
        payload = dict(
            json.loads(FIXTURE.read_text().splitlines()[0]),
            result='<img src=x onerror="window.pwned=1">',
            task_id="<script>window.pwned=2</script>",
        )
        page.fill("#ledger-input", json.dumps(payload))
        page.click("#import-button")
        expect(page.locator("#task-list option")).to_have_count(1)
        # A source task ID containing slashes is also expected to be
        # inspectable;
        # source text in option rendering must already be inert.
        assert (
            page.locator("#task-list")
            .inner_text()
            .startswith(payload["task_id"])
        )
        assert page.evaluate("window.pwned") is None
        # Every accepted ID must remain usable in a browser, including URL dot
        # segments. The query companion uses source url_encode, never
        # raw paths.
        for task_id in [".", "..", "a/b?#%é😀", payload["task_id"]]:
            payload["task_id"] = task_id
            page.fill("#ledger-input", json.dumps(payload, ensure_ascii=False))
            page.click("#import-button")
            expect(page.locator("#status")).to_contain_text(
                "Imported 1 entries"
            )
            page.select_option("#task-list", "0")
            expect(page.locator("#task-detail h2")).to_have_text(task_id)
            expect(page.locator("#task-detail")).to_contain_text(
                payload["result"]
            )
            page.fill("#note-input", "Encoded ID annotation")
            page.click("#save-note")
            expect(page.locator("#status")).to_contain_text("Annotation saved")
        # Inspect the required result payload on a normal task ID.
        payload["task_id"] = "XSS"
        page.fill("#ledger-input", json.dumps(payload))
        page.click("#import-button")
        expect(page.locator("#status")).to_contain_text("Imported 1 entries")
        page.select_option("#task-list", "0")
        expect(page.locator("#task-detail")).to_contain_text(payload["result"])
        assert page.locator("#task-detail img").count() == 0
        assert page.evaluate("window.pwned") is None
        assert errors == [], errors
        print(
            "PASS real Chromium",
            browser.version,
            "import/header/filter/ordered "
            "claims/notes/reload/errors/XSS/keyboard",
        )
        browser.close()


def board_consumer(root):
    source = BOARD / "backend/server.howl"
    fixture = BOARD / "data/fixtures/ledger_sample.jsonl"
    print(
        "Board commit:",
        os.environ.get("HOWLBOARD_COMMIT")
        or command(["git", "-C", BOARD, "rev-parse", "HEAD"]).strip(),
    )
    print(
        "Board sanitized fixture SHA256:",
        hashlib.sha256(fixture.read_bytes()).hexdigest(),
    )
    artifact = root / "board.hfbc"
    original = source.read_bytes()
    marker = b"(http_server 8080"
    insertion = b'\n  (grants "database" "inherit:filesystem")'
    assert original.count(marker) == 1
    adapted = original.replace(marker, marker + insertion, 1)
    adapted_source = root / "board-adapted.howl"
    adapted_source.write_bytes(adapted)
    print(
        "Board original source SHA256:", hashlib.sha256(original).hexdigest()
    )
    print(
        "Board adapter insertion SHA256:",
        hashlib.sha256(insertion).hexdigest(),
    )
    print("Board adapted source SHA256:", hashlib.sha256(adapted).hexdigest())
    command([BIN, "build", adapted_source, "-o", artifact])
    for index, ledger in enumerate([FIXTURE, fixture]):
        records = [
            json.loads(line)
            for line in ledger.read_text().splitlines()
            if line.strip()
        ]
        out = root / f"board-{index}"
        out.mkdir()
        command([APP / "scripts/project.sh", ledger, out])
        base = "http://127.0.0.1:8080"
        caps = f"network,database,filesystem:read={out},filesystem:write={out}"
        with running(
            [
                BIN,
                "run",
                "--listen-host",
                "127.0.0.1",
                "--allow-caps",
                caps,
                "--max-instructions",
                "10000000",
                artifact,
            ],
            out,
            ENV,
            lambda: request(base, "GET", "/api/missions")[0] == 200,
        ):
            listing = request(base, "GET", "/api/missions")
            assert listing[0] == 200
            ids = list(dict.fromkeys(e["task_id"] for e in records))
            total = 0
            for task_id in ids:
                status, result = request(
                    base, "POST", "/api/missions/get", {"id": task_id}
                )
                assert status == 200, result
                # Board returns its mission and ordered
                # timeline as native JSON.
                mission = result
                events = result["timeline"]
                assert (
                    mission["id"] == task_id
                    and mission["provenance"] == "LEDGER"
                )
                assert mission["authority"]["decision"] == "REQUIRE_APPROVAL"
                expected = [e for e in records if e["task_id"] == task_id]
                assert len(events) == len(expected), (task_id, result)
                assert [e["action"] for e in events] == [
                    e["action"] for e in expected
                ]
                total += len(events)
                if index == 0 and task_id == "REVIEW-1":
                    assert (
                        mission["origin"] == "test_fixture"
                        and mission["state"] == "VERIFYING"
                    )
                    assert mission["verification"][0]["status"] == "claimed"
                if index == 0 and task_id == "REVIEW-2":
                    assert mission["state"] == "COMPLETED"
            assert total == len(records)
        print(
            "PASS actual Board consumer",
            ledger.name,
            "records=",
            total,
            "tasks=",
            len(ids),
        )


def capability_and_resource(root):
    data = root / "caps.json"
    port = free_port()
    env = dict(
        ENV,
        EVIDENCEDESK_PORT=str(port),
        EVIDENCEDESK_DATA=str(data),
        EVIDENCEDESK_STATIC=str(APP / "static"),
    )
    artifact = APP / "build/backend.hfbc"
    grants = [
        "network",
        "environment",
        "database",
        f"filesystem:read={data}",
        f"filesystem:write={data}",
        f'filesystem:read={APP / "static"}',
    ]
    # Begin from valid persisted state, then demonstrate each denial without
    # altering those bytes. Static-read denial is exercised by an HTTP request.
    with desk(data) as (base, _):
        assert (
            request(
                base,
                "POST",
                "/api/import",
                FIXTURE.read_bytes(),
                "application/x-ndjson",
            )[0]
            == 200
        )
    before = data.read_bytes()
    for denied in grants[:5]:
        allowed = ",".join(g for g in grants if g != denied)
        result = subprocess.run(
            [
                str(BIN),
                "run",
                "--listen-host",
                "127.0.0.1",
                "--allow-caps",
                allowed,
                str(artifact),
            ],
            env=env,
            capture_output=True,
            text=True,
            timeout=10,
        )
        assert (
            result.returncode != 0 and "CAPABILITY_DENIED" in result.stderr
        ), (denied, result)
        assert data.read_bytes() == before
        print("PASS denied", denied.split("=")[0])
    restricted = ",".join(
        g for g in grants if g != f'filesystem:read={APP / "static"}'
    )
    base = f"http://127.0.0.1:{port}"
    with running(
        [
            BIN,
            "run",
            "--listen-host",
            "127.0.0.1",
            "--allow-caps",
            restricted,
            artifact,
        ],
        APP,
        env,
        lambda: request(base, "GET", "/api/health")[0] == 200,
    ):
        assert request(base, "GET", "/")[0] == 500
    assert data.read_bytes() == before
    print("PASS denied static filesystem read without persisted corruption")
    result = subprocess.run(
        [
            str(BIN),
            "run",
            "--allow-caps",
            ",".join(grants),
            "--max-instructions",
            "1",
            str(artifact),
        ],
        env=env,
        capture_output=True,
        text=True,
        timeout=10,
    )
    assert (
        result.returncode != 0 and "LIMIT_EXCEEDED" in result.stderr
    ), result.stderr
    assert data.read_bytes() == before
    print(
        "PASS too-small finite instruction limit; process and "
        "remote-fetch grants absent throughout"
    )


def main():
    print("Implementation verification compiler:", BIN, flush=True)
    with tempfile.TemporaryDirectory(
        prefix="evidencedesk-verify-"
    ) as temporary:
        root = Path(temporary)
        api_and_restart(root)
        browser(root)
        board_consumer(root)
        capability_and_resource(root)
    print("PASS all implementation workflows")


if __name__ == "__main__":
    main()
