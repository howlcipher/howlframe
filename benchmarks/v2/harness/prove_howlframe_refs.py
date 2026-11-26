#!/usr/bin/env python3
"""Run Track A verification over densified HowlFrame reference implementations.

Fails unless every supported HowlFrame reference task passes and records tokens.
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
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


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group()
    group.add_argument(
        "--check",
        action="store_true",
        help="verify in a temporary directory without updating tracked results/ (default)",
    )
    group.add_argument(
        "--write",
        action="store_true",
        help="update tracked benchmarks/v2/results/reference/howlframe/*.json files",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    env = os.environ.copy()
    tmp_bin_dir: tempfile.TemporaryDirectory[str] | None = None
    tmp_out_dir: tempfile.TemporaryDirectory[str] | None = None
    howl = ROOT / "howlframe"
    try:
        if howl.exists():
            expected_bin = str(howl)
            env["PATH"] = f"{ROOT}:{env.get('PATH', '')}"
        else:
            tmp_bin_dir = tempfile.TemporaryDirectory(prefix="howlframe-v2-proof-")
            built = Path(tmp_bin_dir.name) / "howlframe"
            subprocess.run(["go", "build", "-o", str(built), "howlframe.go"], cwd=ROOT, check=True)
            expected_bin = str(built)
            env["PATH"] = f"{tmp_bin_dir.name}:{env.get('PATH', '')}"

        resolved = shutil.which("howlframe", path=env["PATH"])
        if resolved != expected_bin:
            raise RuntimeError(f"expected howlframe at {expected_bin}, resolved {resolved!r}")

        if args.write:
            out_dir = RESULTS
            out_dir.mkdir(parents=True, exist_ok=True)
        else:
            tmp_out_dir = tempfile.TemporaryDirectory(prefix="howlframe-v2-results-")
            out_dir = Path(tmp_out_dir.name)

        failures = []
        rows = []
        for task, filename in TASKS:
            ref = ROOT / "benchmarks/v2/reference" / task / "howlframe"
            source = str(ref if filename is None else ref / filename)
            out = out_dir / f"{task}.json"
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

        if args.write:
            summary = out_dir / "summary.json"
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
    finally:
        if tmp_out_dir is not None:
            tmp_out_dir.cleanup()
        if tmp_bin_dir is not None:
            tmp_bin_dir.cleanup()


if __name__ == "__main__":
    sys.exit(main())
