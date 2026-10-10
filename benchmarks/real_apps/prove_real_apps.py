#!/usr/bin/env python3
"""Real-app token + functionality proof.

Compares densified dogfooded HowlFrame apps against idiomatic Python/Go/Node
ports of the same contracts. Fails unless:

1. Existing HowlFrame app tests still pass (functionality preserved)
2. Cross-language parity scenarios pass for every port
3. HowlFrame source tokens are under Python for every app and under every
   language on the total
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent

try:
    import tiktoken

    ENC = tiktoken.get_encoding("cl100k_base")

    def count_tokens(text: str) -> int:
        return len(ENC.encode(text))

    METHOD = "tiktoken(cl100k_base)"
except Exception:
    def count_tokens(text: str) -> int:
        return (len(text) + 3) // 4

    METHOD = "character-estimate"


APPS = {
    "log_analyzer": {
        "howlframe": ROOT / "apps/log_analyzer/log_analyzer.howl",
        "python": HERE / "log_analyzer/python/main.py",
        "go": HERE / "log_analyzer/go/main.go",
        "node": HERE / "log_analyzer/node/main.js",
        "kind": "cli_log",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/log_analyzer/",
    },
    "kv_cli": {
        "howlframe": ROOT / "apps/kv_cli/kv_cli.howl",
        "python": HERE / "kv_cli/python/main.py",
        "go": HERE / "kv_cli/go/main.go",
        "node": HERE / "kv_cli/node/main.js",
        "kind": "cli_kv",
        "howl_caps": "database",
        "test_pkg": "./apps/kv_cli/",
    },
    "status_api": {
        "howlframe": ROOT / "apps/status_api/status_api.howl",
        "python": HERE / "status_api/python/main.py",
        "go": HERE / "status_api/go/main.go",
        "node": HERE / "status_api/node/main.js",
        "kind": "http_status",
        "howl_caps": "network,environment",
        "test_pkg": "./apps/status_api/",
    },
    "notes_cli": {
        "howlframe": ROOT / "apps/notes_cli/notes_cli.howl",
        "python": HERE / "notes_cli/python/main.py",
        "go": HERE / "notes_cli/go/main.go",
        "node": HERE / "notes_cli/node/main.js",
        "kind": "cli_notes",
        "howl_caps": "database,filesystem,environment",
        "test_pkg": "./apps/notes_cli/",
    },
    "auth_api": {
        "howlframe": ROOT / "apps/auth_api/auth_api.howl",
        "python": HERE / "auth_api/python/main.py",
        "go": HERE / "auth_api/go/main.go",
        "node": HERE / "auth_api/node/main.js",
        "kind": "http_auth",
        "howl_caps": "network",
        "test_pkg": "./apps/auth_api/",
    },
    "grep_cli": {
        "howlframe": ROOT / "apps/grep_cli/grep_cli.howl",
        "python": HERE / "grep_cli/python/main.py",
        "go": HERE / "grep_cli/go/main.go",
        "node": HERE / "grep_cli/node/main.js",
        "kind": "cli_grep",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/grep_cli/",
    },
}

LANGS = ["howlframe", "python", "go", "node"]


def ensure_howlframe() -> str:
    candidate = ROOT / "howlframe"
    if candidate.exists():
        return str(candidate)
    built = Path(tempfile.gettempdir()) / "howlframe-real-apps"
    subprocess.run(["go", "build", "-o", str(built), "howlframe.go"], cwd=ROOT, check=True)
    return str(built)


def run(cmd, **kwargs):
    return subprocess.run(cmd, capture_output=True, text=True, **kwargs)


def compile_howl(bin_path: str, source: Path, out: Path) -> None:
    r = run([bin_path, "-compile-bc", str(source), "-o", str(out)])
    if r.returncode != 0:
        raise RuntimeError(f"compile {source.name}: {r.stderr or r.stdout}")


def run_cli_log(lang: str, source: Path, howl_bin: str, caps: str, log_path: Path, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "app.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), str(log_path)], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), str(log_path)], cwd=cwd)
    if lang == "go":
        exe = cwd / "log_analyzer"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), str(log_path)], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), str(log_path)], cwd=cwd)
    raise ValueError(lang)


def run_cli_kv(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "kv.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "kv_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def check_http(base: str) -> None:
    with urllib.request.urlopen(base + "/health") as resp:
        body = resp.read().decode()
        if '"status":"ok"' not in body.replace(" ", ""):
            # tolerate spaced JSON
            if '"status"' not in body or "ok" not in body:
                raise RuntimeError(f"/health bad: {body}")
    with urllib.request.urlopen(base + "/ready") as resp:
        body = resp.read().decode()
        if "ready" not in body:
            raise RuntimeError(f"/ready bad: {body}")
    with urllib.request.urlopen(base + "/config") as resp:
        body = resp.read().decode()
        if "foo" not in body:
            raise RuntimeError(f"/config bad: {body}")
    with urllib.request.urlopen(base + "/version") as resp:
        body = resp.read().decode()
        if "1.0.0" not in body:
            raise RuntimeError(f"/version bad: {body}")


def run_http_status(lang: str, source: Path, howl_bin: str, caps: str, cwd: Path):
    env = os.environ.copy()
    env["REQUIRED_ENV"] = "yes"
    env["PUBLIC_CONFIG"] = "foo"
    if lang == "howlframe":
        bc = cwd / "status.hfbc"
        compile_howl(howl_bin, source, bc)
        proc = subprocess.Popen(
            [howl_bin, "-run-bc", "-allow-caps", caps, str(bc)],
            cwd=cwd, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
    elif lang == "python":
        proc = subprocess.Popen(
            [sys.executable, str(source)],
            cwd=cwd, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
    elif lang == "go":
        exe = cwd / "status_api"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        proc = subprocess.Popen([str(exe)], cwd=cwd, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    elif lang == "node":
        proc = subprocess.Popen(["node", str(source)], cwd=cwd, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    else:
        raise ValueError(lang)

    try:
        deadline = time.time() + 3
        last_err = None
        while time.time() < deadline:
            try:
                check_http("http://127.0.0.1:8090")
                return
            except Exception as e:
                last_err = e
                time.sleep(0.1)
        raise RuntimeError(f"http checks failed: {last_err}")
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=2)
        except Exception:
            proc.kill()


def parity_log_analyzer(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        clean = cwd / "clean.log"
        clean.write_text("INFO: Start\nINFO: End\n")
        r = run_cli_log(lang, source, howl_bin, caps, clean, cwd)
        if r.returncode != 0 or "status=CLEAN" not in (r.stdout + r.stderr):
            raise RuntimeError(f"{lang} clean failed: {r.returncode} {r.stdout}{r.stderr}")

        err = cwd / "err.log"
        err.write_text("ERROR: Failed\n")
        r = run_cli_log(lang, source, howl_bin, caps, err, cwd)
        if r.returncode != 1 or "status=ATTENTION" not in (r.stdout + r.stderr):
            raise RuntimeError(f"{lang} err failed: {r.returncode} {r.stdout}{r.stderr}")

        r = run_cli_log(lang, source, howl_bin, caps, cwd / "missing.log", cwd)
        if r.returncode != 2 or "error=" not in (r.stdout + r.stderr):
            raise RuntimeError(f"{lang} missing failed: {r.returncode} {r.stdout}{r.stderr}")


def parity_kv(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        r = run_cli_kv(lang, source, howl_bin, caps,
                       ["set", "theme", "dark", "get", "theme", "get", "missing", "delete", "theme", "get", "theme"],
                       cwd)
        expected = "OK\ndark\nNOT FOUND\nOK\nNOT FOUND\n"
        if r.returncode != 0 or r.stdout != expected:
            raise RuntimeError(f"{lang} kv sequence failed: {r.returncode} {r.stdout!r}")

        r = run_cli_kv(lang, source, howl_bin, caps,
                       ["increment", "visits", "increment", "visits", "get", "visits"], cwd)
        expected = "1\n2\n2\n"
        if r.returncode != 0 or r.stdout != expected:
            raise RuntimeError(f"{lang} kv increment failed: {r.returncode} {r.stdout!r}")


def parity_status(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        run_http_status(lang, source, howl_bin, caps, Path(td))


def run_cli_notes(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path, store: Path):
    env = os.environ.copy()
    env["NOTES_STORE"] = "file://" + str(store)
    if lang == "howlframe":
        bc = cwd / "notes.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd, env=env)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd, env=env)
    if lang == "go":
        exe = cwd / "notes_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd, env=env)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd, env=env)
    raise ValueError(lang)


def parity_notes(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        store = cwd / "notes.store"
        r = run_cli_notes(lang, source, howl_bin, caps,
                          ["add", "first", "add", "second", "list"], cwd, store)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "ADDED 1 first" not in out or "ADDED 2 second" not in out \
                or "1 first" not in out or "2 second" not in out:
            raise RuntimeError(f"{lang} notes add/list failed: {r.returncode} {out!r}")

        r = run_cli_notes(lang, source, howl_bin, caps,
                          ["get", "1", "delete", "2", "list"], cwd, store)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "1 first" not in out or "DELETED 2" not in out or "2 second" in out:
            raise RuntimeError(f"{lang} notes persist failed: {r.returncode} {out!r}")

        r = run_cli_notes(lang, source, howl_bin, caps, ["get", "missing"], cwd, store)
        if "NOT FOUND" not in (r.stdout + r.stderr):
            raise RuntimeError(f"{lang} notes missing failed: {r.returncode} {r.stdout!r}")


def run_cli_grep(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "grep.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "grep_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_grep(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.txt"
        sample.write_text("alpha\nbeta foo\ngamma\nfoo bar\n")
        r = run_cli_grep(lang, source, howl_bin, caps, ["foo", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "beta foo" not in out or "foo bar" not in out or "hits=2" not in out:
            raise RuntimeError(f"{lang} grep hits failed: {r.returncode} {out!r}")

        r = run_cli_grep(lang, source, howl_bin, caps, ["zzz", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode == 0 or "hits=0" not in out:
            raise RuntimeError(f"{lang} grep miss failed: {r.returncode} {out!r}")


def check_auth(base: str) -> None:
    # unauth secret
    try:
        urllib.request.urlopen(base + "/secret")
        raise RuntimeError("secret without auth should 401")
    except urllib.error.HTTPError as e:
        if e.code != 401:
            raise RuntimeError(f"secret unauth want 401 got {e.code}")
    # auth secret
    req = urllib.request.Request(base + "/secret", headers={"Authorization": "Bearer secret"})
    with urllib.request.urlopen(req) as resp:
        body = resp.read().decode()
        if '"ok"' not in body or "1" not in body:
            raise RuntimeError(f"/secret bad: {body}")
    # echo
    req = urllib.request.Request(
        base + "/echo/alice?q=hi", headers={"Authorization": "Bearer secret"}
    )
    with urllib.request.urlopen(req) as resp:
        body = resp.read().decode()
        if "alice" not in body or "hi" not in body:
            raise RuntimeError(f"/echo bad: {body}")
    with urllib.request.urlopen(base + "/health") as resp:
        if "ok" not in resp.read().decode():
            raise RuntimeError("/health bad")


def run_http_auth(lang: str, source: Path, howl_bin: str, caps: str, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "auth.hfbc"
        compile_howl(howl_bin, source, bc)
        proc = subprocess.Popen(
            [howl_bin, "-run-bc", "-allow-caps", caps, str(bc)],
            cwd=cwd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
    elif lang == "python":
        proc = subprocess.Popen(
            [sys.executable, str(source)],
            cwd=cwd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
    elif lang == "go":
        exe = cwd / "auth_api"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        proc = subprocess.Popen([str(exe)], cwd=cwd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    elif lang == "node":
        proc = subprocess.Popen(["node", str(source)], cwd=cwd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    else:
        raise ValueError(lang)

    try:
        deadline = time.time() + 3
        last_err = None
        while time.time() < deadline:
            try:
                check_auth("http://127.0.0.1:8102")
                return
            except Exception as e:
                last_err = e
                time.sleep(0.1)
        raise RuntimeError(f"auth checks failed: {last_err}")
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=2)
        except Exception:
            proc.kill()


def parity_auth(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        run_http_auth(lang, source, howl_bin, caps, Path(td))


def run_app_tests(pkg: str) -> None:
    r = run(["go", "test", pkg, "-count=1"], cwd=ROOT)
    if r.returncode != 0:
        raise RuntimeError(f"go test {pkg} failed:\n{r.stdout}\n{r.stderr}")


def main() -> int:
    howl_bin = ensure_howlframe()
    errors: list[str] = []
    tokens: dict[str, dict[str, int]] = {}
    parity_ok: dict[str, dict[str, bool]] = {}

    print(f"Token method: {METHOD}")
    print(f"{'app':14} {'howl':>6} {'py':>6} {'go':>6} {'node':>6}")

    for app, meta in APPS.items():
        tokens[app] = {}
        parity_ok[app] = {}
        for lang in LANGS:
            tokens[app][lang] = count_tokens(meta[lang].read_text())

        print(f"{app:14} {tokens[app]['howlframe']:6} {tokens[app]['python']:6} "
              f"{tokens[app]['go']:6} {tokens[app]['node']:6}")

        # Functionality: dogfood tests for HowlFrame source of truth.
        try:
            run_app_tests(meta["test_pkg"])
        except Exception as e:
            errors.append(f"{app} howlframe app test: {e}")

        # Parity for every language.
        parity_fn = {
            "cli_log": parity_log_analyzer,
            "cli_kv": parity_kv,
            "http_status": parity_status,
            "cli_notes": parity_notes,
            "http_auth": parity_auth,
            "cli_grep": parity_grep,
        }[meta["kind"]]
        for lang in LANGS:
            try:
                parity_fn(lang, meta[lang], howl_bin, meta["howl_caps"])
                parity_ok[app][lang] = True
            except Exception as e:
                parity_ok[app][lang] = False
                errors.append(f"{app}/{lang} parity: {e}")

        if tokens[app]["howlframe"] >= tokens[app]["python"]:
            errors.append(
                f"{app}: HowlFrame {tokens[app]['howlframe']} not under Python {tokens[app]['python']}"
            )

    totals = {lang: sum(tokens[app][lang] for app in APPS) for lang in LANGS}
    print(f"{'TOTAL':14} {totals['howlframe']:6} {totals['python']:6} {totals['go']:6} {totals['node']:6}")

    for lang in LANGS:
        if lang == "howlframe":
            continue
        if totals["howlframe"] >= totals[lang]:
            errors.append(f"TOTAL: HowlFrame {totals['howlframe']} not under {lang} {totals[lang]}")

    proof = {
        "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "token_method": METHOD,
        "tokens": tokens,
        "totals": totals,
        "parity": parity_ok,
        "errors": errors,
        "passed": len(errors) == 0,
        "notes": (
            "HowlFrame sources are the densified dogfooded apps under apps/. "
            "Ports implement the same behavioral contracts used by apps/*_test.go."
        ),
    }
    out = HERE / "proof_latest.json"
    out.write_text(json.dumps(proof, indent=2) + "\n")

    if errors:
        print("\nFAILED:")
        for e in errors:
            print(f"  - {e}")
        return 1
    print("\nPASSED: densified real apps beat Python/Go/Node on tokens with parity + app tests.")
    print(f"Wrote {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
