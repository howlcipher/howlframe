#!/usr/bin/env python3
"""Additional non-frozen Milestone 1 probes.

Uses independently built artifacts.
"""

import json
import os
from pathlib import Path
import subprocess
import tempfile

BIN = os.environ["HF_BIN"]
ARTIFACTS = Path(os.environ["M1_ARTIFACT_DIR"])
FLAGS = [
    "--max-instructions",
    "4000000000",
    "--max-memory-bytes",
    "4294967296",
    "--max-call-depth",
    "10000",
]


def run(tool, args, source="", grants=""):
    cmd = [BIN, "run", *FLAGS]
    if grants:
        cmd += ["--allow-caps", grants]
    return subprocess.run(
        cmd + [str(ARTIFACTS / (tool + ".hfbc")), *args],
        input=source,
        text=True,
        capture_output=True,
        timeout=30,
    )


with tempfile.TemporaryDirectory() as tmp:
    root = Path(tmp)
    rules = root / "rules.ini"
    # Arbitrarily long budgets must neither overflow nor acquire a hidden cap.
    for budget in [
        "1234567890123.45",
        "9" * 100 + ".1",
        "000000000000000000001.20",
    ]:
        rules.write_text("[budgets]\nUncategorized = " + budget + "\n")
        p = run(
            "ledgerlens",
            ["summary", "--rules", str(rules), "--format", "json", "-"],
            "date,description,amount\n2026-01-01,x,-1.20\n",
            "filesystem:read=" + tmp,
        )
        assert p.returncode == 0, (budget, p)
        row = json.loads(p.stdout)["budgets"][0]
        ip, fp = budget.split(".")
        assert row["limit"] == str(int(ip)) + "." + fp.ljust(2, "0"), row
        assert row["status"] == "OK", row
    rules.write_text("[budgets]\nUncategorized = 000.00\n")
    p = run(
        "ledgerlens",
        ["check-rules", str(rules)],
        grants="filesystem:read=" + tmp,
    )
    assert p.returncode == 65 and "line 2:" in p.stderr, p
    rules.write_text("[budgets]\nUncategorized = 1.19\n")
    p = run(
        "ledgerlens",
        ["summary", "--rules", str(rules), "-"],
        "date,description,amount\n2026-01-01,x,-1.20\n",
        "filesystem:read=" + tmp,
    )
    assert p.returncode == 1 and "\tOVER\n" in p.stdout, p
    # Nested set values in dry-run must never read environment or need
    # its grant.
    source = (
        "set A = ${B}\nset B = ${env:M1_SECRET}\ntask default\n"
        '  run /absolute/program "${A}"\n'
    )
    p = run("taskflow", ["run", "--dry-run", "-f", "-"], source)
    assert p.returncode == 0 and "'${env:M1_SECRET}'" in p.stdout, p
    assert not p.stderr, p
    p = run("taskflow", ["run", "-f", "-"], source)
    assert p.returncode == 77 and not p.stdout, p
    # Decimal CLI options specify values, without a hidden lexical digit cap.
    p = run(
        "ledgerlens",
        ["summary", "--max-rows", "000000000000001", "-"],
        "date,description,amount\n2026-01-01,x,1\n",
    )
    assert p.returncode == 0, p
    history = root / "history.json"
    history.write_text('{"version":1,"runs":[]}')
    p = run(
        "taskflow",
        ["history", "--last", "9" * 100, str(history)],
        grants="filesystem:read=" + tmp,
    )
    assert p.returncode == 0 and not p.stdout, p
    # A dependency cycle at an earlier line must beat a later syntax problem.
    p = run(
        "taskflow",
        ["plan", "-f", "-"],
        "task a after b\ntask b after a\nbogus\n",
    )
    assert (
        p.returncode == 65 and "-:1: dependency cycle: a -> b -> a" in p.stderr
    ), p
    # Rotate recursive variable cycles to the first declaration in the file.
    p = run(
        "taskflow",
        ["run", "--dry-run", "-f", "-"],
        "set A = ${B}\nset B = ${A}\ntask default\n  run /program ${B}\n",
    )
    assert (
        p.returncode == 65 and "-:1: variable cycle: A -> B -> A" in p.stderr
    ), p
    p = run(
        "taskflow",
        ["run", "--dry-run", "-f", "-"],
        "task\tdefault\n\trun\t/program\targument\n",
    )
    assert (
        p.returncode == 0 and "| would run: /program argument" in p.stdout
    ), p
    # Earlier config failures must beat later unknown keys; no network
    # grant used.
    config = {
        "version": 1,
        "checks": [
            {
                "name": "x",
                "url": "http://localhost/",
                "method": "HEAD",
                "body_contains": "x",
                "unknown": 1,
            }
        ],
        "unknown": 1,
    }
    p = run("healthwatch", ["validate", "-"], json.dumps(config))
    assert p.returncode == 65 and "checks[0].body_contains:" in p.stderr, p
    state = root / "state.json"
    state.write_text('{"version":1,"checks":{}}')
    config = {
        "version": 1,
        "checks": [{"name": "x", "url": "http://localhost/"}],
    }
    p = run(
        "healthwatch",
        ["run", "--state", str(state), "-"],
        json.dumps(config),
        "filesystem:read=" + tmp,
    )
    assert p.returncode == 77 and not p.stdout, p
    assert state.read_text() == '{"version":1,"checks":{}}', p
print(
    "PASS: uncapped budgets, nested dry-run environment, config "
    "precedence, state denial"
)
