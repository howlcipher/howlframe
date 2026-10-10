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
    "wc_cli": {
        "howlframe": ROOT / "apps/wc_cli/wc_cli.howl",
        "python": HERE / "wc_cli/python/main.py",
        "go": HERE / "wc_cli/go/main.go",
        "node": HERE / "wc_cli/node/main.js",
        "kind": "cli_wc",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/wc_cli/",
    },
    "json_get": {
        "howlframe": ROOT / "apps/json_get/json_get.howl",
        "python": HERE / "json_get/python/main.py",
        "go": HERE / "json_get/go/main.go",
        "node": HERE / "json_get/node/main.js",
        "kind": "cli_json",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/json_get/",
    },
    "counter_api": {
        "howlframe": ROOT / "apps/counter_api/counter_api.howl",
        "python": HERE / "counter_api/python/main.py",
        "go": HERE / "counter_api/go/main.go",
        "node": HERE / "counter_api/node/main.js",
        "kind": "http_counter",
        "howl_caps": "network,database",
        "test_pkg": "./apps/counter_api/",
    },
    "fetch_cli": {
        "howlframe": ROOT / "apps/fetch_cli/fetch_cli.howl",
        "python": HERE / "fetch_cli/python/main.py",
        "go": HERE / "fetch_cli/go/main.go",
        "node": HERE / "fetch_cli/node/main.js",
        "kind": "cli_fetch",
        "howl_caps": "network",
        "test_pkg": "./apps/fetch_cli/",
    },
    "template_cli": {
        "howlframe": ROOT / "apps/template_cli/template_cli.howl",
        "python": HERE / "template_cli/python/main.py",
        "go": HERE / "template_cli/go/main.go",
        "node": HERE / "template_cli/node/main.js",
        "kind": "cli_template",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/template_cli/",
    },
    "stamp_api": {
        "howlframe": ROOT / "apps/stamp_api/stamp_api.howl",
        "python": HERE / "stamp_api/python/main.py",
        "go": HERE / "stamp_api/go/main.go",
        "node": HERE / "stamp_api/node/main.js",
        "kind": "http_stamp",
        "howl_caps": "network",
        "test_pkg": "./apps/stamp_api/",
    },
    "mkdir_cli": {
        "howlframe": ROOT / "apps/mkdir_cli/mkdir_cli.howl",
        "python": HERE / "mkdir_cli/python/main.py",
        "go": HERE / "mkdir_cli/go/main.go",
        "node": HERE / "mkdir_cli/node/main.js",
        "kind": "cli_mkdir",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/mkdir_cli/",
    },
    "contains_cli": {
        "howlframe": ROOT / "apps/contains_cli/contains_cli.howl",
        "python": HERE / "contains_cli/python/main.py",
        "go": HERE / "contains_cli/go/main.go",
        "node": HERE / "contains_cli/node/main.js",
        "kind": "cli_contains",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/contains_cli/",
    },
    "file_transform": {
        "howlframe": ROOT / "apps/file_transform/file_transform.howl",
        "python": HERE / "file_transform/python/main.py",
        "go": HERE / "file_transform/go/main.go",
        "node": HERE / "file_transform/node/main.js",
        "kind": "cli_transform",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/file_transform/",
    },
    "env_cli": {
        "howlframe": ROOT / "apps/env_cli/env_cli.howl",
        "python": HERE / "env_cli/python/main.py",
        "go": HERE / "env_cli/go/main.go",
        "node": HERE / "env_cli/node/main.js",
        "kind": "cli_env",
        "howl_caps": "environment",
        "test_pkg": "./apps/env_cli/",
    },
    "starts_cli": {
        "howlframe": ROOT / "apps/starts_cli/starts_cli.howl",
        "python": HERE / "starts_cli/python/main.py",
        "go": HERE / "starts_cli/go/main.go",
        "node": HERE / "starts_cli/node/main.js",
        "kind": "cli_starts",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/starts_cli/",
    },
    "ends_cli": {
        "howlframe": ROOT / "apps/ends_cli/ends_cli.howl",
        "python": HERE / "ends_cli/python/main.py",
        "go": HERE / "ends_cli/go/main.go",
        "node": HERE / "ends_cli/node/main.js",
        "kind": "cli_ends",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/ends_cli/",
    },
    "head_cli": {
        "howlframe": ROOT / "apps/head_cli/head_cli.howl",
        "python": HERE / "head_cli/python/main.py",
        "go": HERE / "head_cli/go/main.go",
        "node": HERE / "head_cli/node/main.js",
        "kind": "cli_head",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/head_cli/",
    },
    "cut_cli": {
        "howlframe": ROOT / "apps/cut_cli/cut_cli.howl",
        "python": HERE / "cut_cli/python/main.py",
        "go": HERE / "cut_cli/go/main.go",
        "node": HERE / "cut_cli/node/main.js",
        "kind": "cli_cut",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/cut_cli/",
    },
    "has_line_cli": {
        "howlframe": ROOT / "apps/has_line_cli/has_line_cli.howl",
        "python": HERE / "has_line_cli/python/main.py",
        "go": HERE / "has_line_cli/go/main.go",
        "node": HERE / "has_line_cli/node/main.js",
        "kind": "cli_has_line",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/has_line_cli/",
    },
    "join_cli": {
        "howlframe": ROOT / "apps/join_cli/join_cli.howl",
        "python": HERE / "join_cli/python/main.py",
        "go": HERE / "join_cli/go/main.go",
        "node": HERE / "join_cli/node/main.js",
        "kind": "cli_join",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/join_cli/",
    },
    "uniq_cli": {
        "howlframe": ROOT / "apps/uniq_cli/uniq_cli.howl",
        "python": HERE / "uniq_cli/python/main.py",
        "go": HERE / "uniq_cli/go/main.go",
        "node": HERE / "uniq_cli/node/main.js",
        "kind": "cli_uniq",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/uniq_cli/",
    },
    "echo_api": {
        "howlframe": ROOT / "apps/echo_api/echo_api.howl",
        "python": HERE / "echo_api/python/main.py",
        "go": HERE / "echo_api/go/main.go",
        "node": HERE / "echo_api/node/main.js",
        "kind": "http_echo",
        "howl_caps": "network",
        "test_pkg": "./apps/echo_api/",
    },
    "now_cli": {
        "howlframe": ROOT / "apps/now_cli/now_cli.howl",
        "python": HERE / "now_cli/python/main.py",
        "go": HERE / "now_cli/go/main.go",
        "node": HERE / "now_cli/node/main.js",
        "kind": "cli_now",
        "howl_caps": "",
        "test_pkg": "./apps/now_cli/",
    },
    "sleep_cli": {
        "howlframe": ROOT / "apps/sleep_cli/sleep_cli.howl",
        "python": HERE / "sleep_cli/python/main.py",
        "go": HERE / "sleep_cli/go/main.go",
        "node": HERE / "sleep_cli/node/main.js",
        "kind": "cli_sleep",
        "howl_caps": "",
        "test_pkg": "./apps/sleep_cli/",
    },
    "lower_cli": {
        "howlframe": ROOT / "apps/lower_cli/lower_cli.howl",
        "python": HERE / "lower_cli/python/main.py",
        "go": HERE / "lower_cli/go/main.go",
        "node": HERE / "lower_cli/node/main.js",
        "kind": "cli_lower",
        "howl_caps": "",
        "test_pkg": "./apps/lower_cli/",
    },
    "upper_cli": {
        "howlframe": ROOT / "apps/upper_cli/upper_cli.howl",
        "python": HERE / "upper_cli/python/main.py",
        "go": HERE / "upper_cli/go/main.go",
        "node": HERE / "upper_cli/node/main.js",
        "kind": "cli_upper",
        "howl_caps": "",
        "test_pkg": "./apps/upper_cli/",
    },
    "trim_cli": {
        "howlframe": ROOT / "apps/trim_cli/trim_cli.howl",
        "python": HERE / "trim_cli/python/main.py",
        "go": HERE / "trim_cli/go/main.go",
        "node": HERE / "trim_cli/node/main.js",
        "kind": "cli_trim",
        "howl_caps": "",
        "test_pkg": "./apps/trim_cli/",
    },
    "icontains_cli": {
        "howlframe": ROOT / "apps/icontains_cli/icontains_cli.howl",
        "python": HERE / "icontains_cli/python/main.py",
        "go": HERE / "icontains_cli/go/main.go",
        "node": HERE / "icontains_cli/node/main.js",
        "kind": "cli_icontains",
        "howl_caps": "filesystem",
        "test_pkg": "./apps/icontains_cli/",
    },
}

LANGS = ["howlframe", "python", "go", "node"]


def ensure_howlframe() -> str:
    # Always rebuild so prove tracks language changes on the branch tip.
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


def run_cli_wc(lang: str, source: Path, howl_bin: str, caps: str, path: Path, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "wc.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), str(path)], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), str(path)], cwd=cwd)
    if lang == "go":
        exe = cwd / "wc_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), str(path)], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), str(path)], cwd=cwd)
    raise ValueError(lang)


def parity_wc(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.txt"
        sample.write_bytes(b"one two\nthree\nfour five six\n")
        r = run_cli_wc(lang, source, howl_bin, caps, sample, cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "lines=4" not in out or "words=6" not in out or "bytes=28" not in out:
            raise RuntimeError(f"{lang} wc failed: {r.returncode} {out!r}")


def run_cli_json(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "json.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "json_get"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_json(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.json"
        sample.write_text('{"name":"ada","n":"7"}')
        r = run_cli_json(lang, source, howl_bin, caps, [str(sample), "name"], cwd)
        if r.returncode != 0 or "ada" not in (r.stdout + r.stderr):
            raise RuntimeError(f"{lang} json name failed: {r.returncode} {r.stdout!r}")
        r = run_cli_json(lang, source, howl_bin, caps, [str(sample), "n"], cwd)
        if r.returncode != 0 or "7" not in (r.stdout + r.stderr):
            raise RuntimeError(f"{lang} json n failed: {r.returncode} {r.stdout!r}")
        r = run_cli_json(lang, source, howl_bin, caps, [str(sample), "missing"], cwd)
        if r.returncode == 0 or "NOT FOUND" not in (r.stdout + r.stderr):
            raise RuntimeError(f"{lang} json missing failed: {r.returncode} {r.stdout!r}")


def check_counter(base: str) -> None:
    with urllib.request.urlopen(base + "/count") as resp:
        body = resp.read().decode()
        if '"n":"0"' not in body.replace(" ", ""):
            raise RuntimeError(f"/count want 0: {body}")
    req = urllib.request.Request(base + "/inc", method="POST")
    with urllib.request.urlopen(req) as resp:
        body = resp.read().decode()
        if '"n":"1"' not in body.replace(" ", ""):
            raise RuntimeError(f"/inc want 1: {body}")
    req = urllib.request.Request(base + "/inc", method="POST")
    with urllib.request.urlopen(req) as resp:
        body = resp.read().decode()
        if '"n":"2"' not in body.replace(" ", ""):
            raise RuntimeError(f"/inc want 2: {body}")
    req = urllib.request.Request(base + "/reset", method="POST")
    with urllib.request.urlopen(req) as resp:
        body = resp.read().decode()
        if '"n":"0"' not in body.replace(" ", ""):
            raise RuntimeError(f"/reset want 0: {body}")


def run_http_counter(lang: str, source: Path, howl_bin: str, caps: str, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "counter.hfbc"
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
        exe = cwd / "counter_api"
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
                check_counter("http://127.0.0.1:8103")
                return
            except Exception as e:
                last_err = e
                time.sleep(0.1)
        raise RuntimeError(f"counter checks failed: {last_err}")
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=2)
        except Exception:
            proc.kill()


def parity_counter(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        run_http_counter(lang, source, howl_bin, caps, Path(td))


def start_fetch_origin():
    """Tiny local origin for fetch_cli parity (418 + JSON body)."""
    import threading
    from http.server import BaseHTTPRequestHandler, HTTPServer

    class H(BaseHTTPRequestHandler):
        def log_message(self, fmt, *args):
            return

        def do_GET(self):
            body = b'{"ok":"1"}'
            self.send_response(418)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    srv = HTTPServer(("127.0.0.1", 0), H)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    return srv, f"http://127.0.0.1:{srv.server_port}"


def run_cli_fetch(lang: str, source: Path, howl_bin: str, caps: str, url: str, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "fetch.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), url], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), url], cwd=cwd)
    if lang == "go":
        exe = cwd / "fetch_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), url], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), url], cwd=cwd)
    raise ValueError(lang)


def parity_fetch(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    srv, url = start_fetch_origin()
    try:
        with tempfile.TemporaryDirectory() as td:
            r = run_cli_fetch(lang, source, howl_bin, caps, url, Path(td))
            out = r.stdout + r.stderr
            if r.returncode != 0 or "status=418" not in out or '"ok":"1"' not in out.replace(" ", ""):
                raise RuntimeError(f"{lang} fetch failed: {r.returncode} {out!r}")
    finally:
        srv.shutdown()


def run_cli_template(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "tpl.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "template_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_template(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        tpl = cwd / "tpl.txt"
        tpl.write_text("Hi {{name}}!\n")
        r = run_cli_template(lang, source, howl_bin, caps, [str(tpl), "name", "Ada"], cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "Hi Ada!" not in out:
            raise RuntimeError(f"{lang} template failed: {r.returncode} {out!r}")
        nop = cwd / "nop.txt"
        nop.write_text("no tokens\n")
        r = run_cli_template(lang, source, howl_bin, caps, [str(nop), "name", "Ada"], cwd)
        if r.returncode == 0 or "NO PLACEHOLDER" not in (r.stdout + r.stderr):
            raise RuntimeError(f"{lang} template miss failed: {r.returncode} {r.stdout!r}")


def check_stamp(base: str) -> None:
    with urllib.request.urlopen(base + "/health") as resp:
        if "ok" not in resp.read().decode():
            raise RuntimeError("/health bad")
    with urllib.request.urlopen(base + "/now") as resp:
        body = resp.read().decode()
        if "unix" not in body:
            raise RuntimeError(f"/now bad: {body}")
    with urllib.request.urlopen(base + "/echo/ada") as resp:
        body = resp.read().decode()
        if "ada" not in body or "unix" not in body:
            raise RuntimeError(f"/echo bad: {body}")


def run_http_stamp(lang: str, source: Path, howl_bin: str, caps: str, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "stamp.hfbc"
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
        exe = cwd / "stamp_api"
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
                check_stamp("http://127.0.0.1:8104")
                return
            except Exception as e:
                last_err = e
                time.sleep(0.1)
        raise RuntimeError(f"stamp checks failed: {last_err}")
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=2)
        except Exception:
            proc.kill()


def parity_stamp(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        run_http_stamp(lang, source, howl_bin, caps, Path(td))


def run_cli_mkdir(lang: str, source: Path, howl_bin: str, caps: str, path: Path, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "mkdir.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), str(path)], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), str(path)], cwd=cwd)
    if lang == "go":
        exe = cwd / "mkdir_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), str(path)], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), str(path)], cwd=cwd)
    raise ValueError(lang)


def parity_mkdir(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        dest = cwd / "nested" / "dir"
        r = run_cli_mkdir(lang, source, howl_bin, caps, dest, cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "OK" not in out:
            raise RuntimeError(f"{lang} mkdir failed: {r.returncode} {out!r}")
        if not dest.is_dir():
            raise RuntimeError(f"{lang} mkdir did not create {dest}")


def run_cli_contains(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "contains.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "contains_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_contains(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.txt"
        sample.write_text("alpha beta gamma\n")
        r = run_cli_contains(lang, source, howl_bin, caps, ["beta", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "FOUND" not in out:
            raise RuntimeError(f"{lang} contains hit failed: {r.returncode} {out!r}")
        r = run_cli_contains(lang, source, howl_bin, caps, ["zzz", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode == 0 or "MISSING" not in out:
            raise RuntimeError(f"{lang} contains miss failed: {r.returncode} {out!r}")


def run_cli_transform(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "transform.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "file_transform"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_transform(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        inp = cwd / "sample.csv"
        outp = cwd / "sample.jsonl"
        inp.write_text("name,age\nalice,30\nbob,25\n")
        r = run_cli_transform(lang, source, howl_bin, caps, [str(inp), str(outp)], cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "rows=3" not in out:
            raise RuntimeError(f"{lang} transform failed: {r.returncode} {out!r}")
        text = outp.read_text()
        if '"c0":"alice"' not in text or '"c1":"30"' not in text:
            raise RuntimeError(f"{lang} transform jsonl bad: {text!r}")
        r = run_cli_transform(lang, source, howl_bin, caps, [str(cwd / "missing.csv"), str(outp)], cwd)
        if r.returncode == 0 or "error=" not in (r.stdout + r.stderr):
            raise RuntimeError(f"{lang} transform missing failed: {r.returncode} {r.stdout!r}")


def run_cli_env(lang: str, source: Path, howl_bin: str, caps: str, key: str, cwd: Path, env: dict):
    if lang == "howlframe":
        bc = cwd / "env.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), key], cwd=cwd, env=env)
    if lang == "python":
        return run([sys.executable, str(source), key], cwd=cwd, env=env)
    if lang == "go":
        exe = cwd / "env_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), key], cwd=cwd, env=env)
    if lang == "node":
        return run(["node", str(source), key], cwd=cwd, env=env)
    raise ValueError(lang)


def parity_env(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        env = os.environ.copy()
        env["HOWLFRAME_ENV_CLI"] = "hello"
        env.pop("HOWLFRAME_ENV_CLI_MISSING", None)
        r = run_cli_env(lang, source, howl_bin, caps, "HOWLFRAME_ENV_CLI", cwd, env)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "hello" not in out:
            raise RuntimeError(f"{lang} env get failed: {r.returncode} {out!r}")
        r = run_cli_env(lang, source, howl_bin, caps, "HOWLFRAME_ENV_CLI_MISSING", cwd, env)
        out = r.stdout + r.stderr
        if r.returncode == 0 or "MISSING" not in out:
            raise RuntimeError(f"{lang} env miss failed: {r.returncode} {out!r}")


def run_cli_starts(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "starts.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "starts_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_starts(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.txt"
        sample.write_text("hello world\n")
        r = run_cli_starts(lang, source, howl_bin, caps, ["hello", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "YES" not in out:
            raise RuntimeError(f"{lang} starts yes failed: {r.returncode} {out!r}")
        r = run_cli_starts(lang, source, howl_bin, caps, ["world", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode == 0 or "NO" not in out:
            raise RuntimeError(f"{lang} starts no failed: {r.returncode} {out!r}")


def run_cli_ends(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "ends.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "ends_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_ends(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.txt"
        sample.write_text("hello world\n")
        r = run_cli_ends(lang, source, howl_bin, caps, ["world\n", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "YES" not in out:
            raise RuntimeError(f"{lang} ends yes failed: {r.returncode} {out!r}")
        r = run_cli_ends(lang, source, howl_bin, caps, ["hello", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode == 0 or "NO" not in out:
            raise RuntimeError(f"{lang} ends no failed: {r.returncode} {out!r}")


def run_cli_head(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "head.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "head_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_head(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.txt"
        sample.write_text("a\nb\nc\nd\n")
        r = run_cli_head(lang, source, howl_bin, caps, ["2", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "a\n" not in out or "b\n" not in out or out.count("c") > 0:
            raise RuntimeError(f"{lang} head failed: {r.returncode} {out!r}")


def run_cli_cut(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "cut.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "cut_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_cut(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.csv"
        sample.write_text("a,b,c\n1,2,3\n")
        r = run_cli_cut(lang, source, howl_bin, caps, ["1", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "b\n" not in out or "2\n" not in out:
            raise RuntimeError(f"{lang} cut failed: {r.returncode} {out!r}")


def run_cli_has_line(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "has_line.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "has_line_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_has_line(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.txt"
        sample.write_text("alpha\nbeta\ngamma\n")
        r = run_cli_has_line(lang, source, howl_bin, caps, ["beta", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "FOUND" not in out:
            raise RuntimeError(f"{lang} has_line hit failed: {r.returncode} {out!r}")
        r = run_cli_has_line(lang, source, howl_bin, caps, ["zzz", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode == 0 or "MISSING" not in out:
            raise RuntimeError(f"{lang} has_line miss failed: {r.returncode} {out!r}")


def run_cli_join(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "join.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "join_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_join(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.txt"
        sample.write_text("a\nb\nc\n")
        r = run_cli_join(lang, source, howl_bin, caps, [",", str(sample)], cwd)
        out = (r.stdout + r.stderr).strip()
        if r.returncode != 0 or out != "a,b,c,":
            raise RuntimeError(f"{lang} join failed: {r.returncode} {out!r}")


def run_cli_uniq(lang: str, source: Path, howl_bin: str, caps: str, path: Path, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "uniq.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), str(path)], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), str(path)], cwd=cwd)
    if lang == "go":
        exe = cwd / "uniq_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), str(path)], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), str(path)], cwd=cwd)
    raise ValueError(lang)


def parity_uniq(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.txt"
        sample.write_text("a\nb\na\nc\nb\n")
        r = run_cli_uniq(lang, source, howl_bin, caps, sample, cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or out.count("a\n") != 1 or out.count("b\n") != 1 or "c\n" not in out:
            raise RuntimeError(f"{lang} uniq failed: {r.returncode} {out!r}")


def check_echo(base: str) -> None:
    with urllib.request.urlopen(base + "/health") as resp:
        if "ok" not in resp.read().decode():
            raise RuntimeError("/health bad")
    with urllib.request.urlopen(base + "/echo?q=hi") as resp:
        body = resp.read().decode()
        if "GET" not in body or "hi" not in body:
            raise RuntimeError(f"/echo bad: {body}")
    with urllib.request.urlopen(base + "/echo/ada") as resp:
        body = resp.read().decode()
        if "ada" not in body:
            raise RuntimeError(f"/echo/name bad: {body}")


def run_http_echo(lang: str, source: Path, howl_bin: str, caps: str, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "echo.hfbc"
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
        exe = cwd / "echo_api"
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
                check_echo("http://127.0.0.1:8106")
                return
            except Exception as e:
                last_err = e
                time.sleep(0.1)
        raise RuntimeError(f"echo checks failed: {last_err}")
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=2)
        except Exception:
            proc.kill()


def parity_echo(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        run_http_echo(lang, source, howl_bin, caps, Path(td))


def run_cli_now(lang: str, source: Path, howl_bin: str, caps: str, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "now.hfbc"
        compile_howl(howl_bin, source, bc)
        cmd = [howl_bin, "-run-bc", str(bc)]
        if caps:
            cmd = [howl_bin, "-run-bc", "-allow-caps", caps, str(bc)]
        return run(cmd, cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source)], cwd=cwd)
    if lang == "go":
        exe = cwd / "now_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe)], cwd=cwd)
    if lang == "node":
        return run(["node", str(source)], cwd=cwd)
    raise ValueError(lang)


def parity_now(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        r = run_cli_now(lang, source, howl_bin, caps, Path(td))
        out = (r.stdout + r.stderr).strip()
        if r.returncode != 0:
            raise RuntimeError(f"{lang} now failed: {r.returncode} {out!r}")
        n = int(out)
        now = int(time.time())
        if abs(n - now) > 5:
            raise RuntimeError(f"{lang} now out of range: {n} vs {now}")


def run_cli_sleep(lang: str, source: Path, howl_bin: str, caps: str, ms: str, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "sleep.hfbc"
        compile_howl(howl_bin, source, bc)
        cmd = [howl_bin, "-run-bc", str(bc), ms]
        if caps:
            cmd = [howl_bin, "-run-bc", "-allow-caps", caps, str(bc), ms]
        return run(cmd, cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), ms], cwd=cwd)
    if lang == "go":
        exe = cwd / "sleep_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), ms], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), ms], cwd=cwd)
    raise ValueError(lang)


def parity_sleep(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        start = time.time()
        r = run_cli_sleep(lang, source, howl_bin, caps, "50", Path(td))
        out = r.stdout + r.stderr
        if r.returncode != 0 or "OK" not in out:
            raise RuntimeError(f"{lang} sleep failed: {r.returncode} {out!r}")
        if time.time() - start < 0.04:
            raise RuntimeError(f"{lang} sleep too brief")


def run_cli_lower(lang: str, source: Path, howl_bin: str, caps: str, text: str, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "lower.hfbc"
        compile_howl(howl_bin, source, bc)
        cmd = [howl_bin, "-run-bc", str(bc), text]
        if caps:
            cmd = [howl_bin, "-run-bc", "-allow-caps", caps, str(bc), text]
        return run(cmd, cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), text], cwd=cwd)
    if lang == "go":
        exe = cwd / "lower_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), text], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), text], cwd=cwd)
    raise ValueError(lang)


def parity_lower(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        r = run_cli_lower(lang, source, howl_bin, caps, "Hello World", Path(td))
        out = (r.stdout + r.stderr).strip()
        if r.returncode != 0 or out != "hello world":
            raise RuntimeError(f"{lang} lower failed: {r.returncode} {out!r}")


def run_cli_upper(lang: str, source: Path, howl_bin: str, caps: str, text: str, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "upper.hfbc"
        compile_howl(howl_bin, source, bc)
        cmd = [howl_bin, "-run-bc", str(bc), text]
        if caps:
            cmd = [howl_bin, "-run-bc", "-allow-caps", caps, str(bc), text]
        return run(cmd, cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), text], cwd=cwd)
    if lang == "go":
        exe = cwd / "upper_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), text], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), text], cwd=cwd)
    raise ValueError(lang)


def parity_upper(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        r = run_cli_upper(lang, source, howl_bin, caps, "Hello World", Path(td))
        out = (r.stdout + r.stderr).strip()
        if r.returncode != 0 or out != "HELLO WORLD":
            raise RuntimeError(f"{lang} upper failed: {r.returncode} {out!r}")


def run_cli_trim(lang: str, source: Path, howl_bin: str, caps: str, text: str, cwd: Path):
    if lang == "howlframe":
        bc = cwd / "trim.hfbc"
        compile_howl(howl_bin, source, bc)
        cmd = [howl_bin, "-run-bc", str(bc), text]
        if caps:
            cmd = [howl_bin, "-run-bc", "-allow-caps", caps, str(bc), text]
        return run(cmd, cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), text], cwd=cwd)
    if lang == "go":
        exe = cwd / "trim_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), text], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), text], cwd=cwd)
    raise ValueError(lang)


def parity_trim(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        r = run_cli_trim(lang, source, howl_bin, caps, "  hello  ", Path(td))
        out = (r.stdout + r.stderr).strip()
        if r.returncode != 0 or out != "hello":
            raise RuntimeError(f"{lang} trim failed: {r.returncode} {out!r}")


def run_cli_icontains(lang: str, source: Path, howl_bin: str, caps: str, args: list[str], cwd: Path):
    if lang == "howlframe":
        bc = cwd / "icontains.hfbc"
        compile_howl(howl_bin, source, bc)
        return run([howl_bin, "-run-bc", "-allow-caps", caps, str(bc), *args], cwd=cwd)
    if lang == "python":
        return run([sys.executable, str(source), *args], cwd=cwd)
    if lang == "go":
        exe = cwd / "icontains_cli"
        r = run(["go", "build", "-o", str(exe), str(source)])
        if r.returncode != 0:
            raise RuntimeError(r.stderr)
        return run([str(exe), *args], cwd=cwd)
    if lang == "node":
        return run(["node", str(source), *args], cwd=cwd)
    raise ValueError(lang)


def parity_icontains(lang: str, source: Path, howl_bin: str, caps: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        cwd = Path(td)
        sample = cwd / "sample.txt"
        sample.write_text("Alpha Beta Gamma\n")
        r = run_cli_icontains(lang, source, howl_bin, caps, ["beta", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode != 0 or "FOUND" not in out:
            raise RuntimeError(f"{lang} icontains found failed: {r.returncode} {out!r}")
        r = run_cli_icontains(lang, source, howl_bin, caps, ["delta", str(sample)], cwd)
        out = r.stdout + r.stderr
        if r.returncode == 0 or "MISSING" not in out:
            raise RuntimeError(f"{lang} icontains missing failed: {r.returncode} {out!r}")


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
            "cli_wc": parity_wc,
            "cli_json": parity_json,
            "http_counter": parity_counter,
            "cli_fetch": parity_fetch,
            "cli_template": parity_template,
            "http_stamp": parity_stamp,
            "cli_mkdir": parity_mkdir,
            "cli_contains": parity_contains,
            "cli_transform": parity_transform,
            "cli_env": parity_env,
            "cli_starts": parity_starts,
            "cli_ends": parity_ends,
            "cli_head": parity_head,
            "cli_cut": parity_cut,
            "cli_has_line": parity_has_line,
            "cli_join": parity_join,
            "cli_uniq": parity_uniq,
            "http_echo": parity_echo,
            "cli_now": parity_now,
            "cli_sleep": parity_sleep,
            "cli_lower": parity_lower,
            "cli_upper": parity_upper,
            "cli_trim": parity_trim,
            "cli_icontains": parity_icontains,
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
