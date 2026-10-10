#!/usr/bin/env python3
"""Run Track A verification over densified HowlFrame reference implementations.

Fails unless every supported HowlFrame reference task passes and records tokens.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
HARNESS = Path(__file__).resolve().parent
RESULTS = ROOT / "benchmarks/v2/results/reference/howlframe"

TASKS = [
    ("task_01_typed_function", "app.howl"),
    ("task_02_cli_file_processing", "app.howl"),
    ("task_03_http_json_endpoint", "server.howl"),
    ("task_04_multi_module_cli", None),  # directory
    ("task_06_error_handling", "app.howl"),
    ("task_07_native_store", "app.howl"),
    ("task_08_capability_restricted", "app.howl"),
]


def main() -> int:
    env = os.environ.copy()
    howl = ROOT / "howlframe"
    if howl.exists():
        env["PATH"] = f"{ROOT}:{env.get('PATH', '')}"
    else:
        built = Path("/tmp/howlframe-v2-proof")
        subprocess.run(["go", "build", "-o", str(built), "howlframe.go"], cwd=ROOT, check=True)
        env["PATH"] = f"{built.parent}:{env.get('PATH', '')}"

    RESULTS.mkdir(parents=True, exist_ok=True)
    failures = []
    rows = []
    for task, filename in TASKS:
        ref = ROOT / "benchmarks/v2/reference" / task / "howlframe"
        source = str(ref if filename is None else ref / filename)
        out = RESULTS / f"{task}.json"
        r = subprocess.run(
            [
                sys.executable,
                str(HARNESS / "run_track_a.py"),
                "--task", task,
                "--language", "howlframe",
                "--model", "reference",
                "--sources", source,
                "--out", str(out),
            ],
            cwd=ROOT,
            env=env,
            capture_output=True,
            text=True,
        )
        if not out.exists():
            failures.append(f"{task}: no result file ({r.stderr or r.stdout})")
            continue
        data = json.loads(out.read_text())
        m = data["metrics"]
        rows.append((task, m["eventual_success"], m["final_source_tokens"]))
        if not m["eventual_success"]:
            failures.append(f"{task}: verification failed ({r.stdout} {r.stderr})")

    print(f"{'task':40} {'pass':5} {'tokens':>6}")
    for task, ok, tokens in rows:
        print(f"{task:40} {str(ok):5} {tokens:6}")

    summary = RESULTS / "summary.json"
    summary.write_text(json.dumps({
        "rows": [{"task": t, "passed": ok, "tokens": n} for t, ok, n in rows],
        "passed": len(failures) == 0,
        "failures": failures,
    }, indent=2) + "\n")

    if failures:
        print("\nFAILED:")
        for f in failures:
            print(f"  - {f}")
        return 1
    print("\nPASSED: all densified HowlFrame v2 references verify.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
