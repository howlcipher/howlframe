#!/usr/bin/env python3
"""Recount write-cost tokens and verify HowlFrame fixtures beat baselines.

Exit 0 only when:
  - every HowlFrame v1 fixture verifies (compile + run where applicable)
  - HowlFrame tokens < Python tokens on tasks A, B, and C
  - HowlFrame total tokens < Python, Node, Go, C#, and Java totals
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BENCH = Path(__file__).resolve().parent

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


TASKS = {
    "task_a_hello_json": {
        "HowlFrame": ["howlframe/hello.howl"],
        "go": ["go/main.go"],
        "python": ["python/main.py"],
        "node": ["node/main.js"],
        "csharp": ["csharp/Program.cs"],
        "java": ["java/Main.java"],
    },
    "task_b_cli_file_sum": {
        "HowlFrame": ["howlframe/greet.howl"],
        "go": ["go/main.go"],
        "python": ["python/main.py"],
        "node": ["node/main.js"],
        "csharp": ["csharp/Program.cs"],
        "java": ["java/Main.java"],
    },
    "task_c_function_test": {
        "HowlFrame": ["howlframe/add.howl"],
        "go": ["go/main.go", "go/main_test.go"],
        "python": ["python/main.py", "python/test_main.py"],
        "node": ["node/main.js", "node/main.test.js"],
        "csharp": ["csharp/Calculator.cs", "csharp/CalculatorTests.cs"],
        "java": ["java/Calculator.java", "java/CalculatorTests.java"],
    },
}

LANG_ORDER = ["HowlFrame", "go", "python", "node", "csharp", "java"]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group()
    group.add_argument(
        "--check",
        action="store_true",
        help="verify counts without writing proof_latest.json (default)",
    )
    group.add_argument(
        "--write",
        action="store_true",
        help="write updated benchmarks/language_write_cost/proof_latest.json",
    )
    return parser.parse_args()


def howlframe_bin() -> tuple[str, tempfile.TemporaryDirectory[str] | None]:
    candidate = ROOT / "howlframe"
    if candidate.exists():
        return str(candidate), None
    td = tempfile.TemporaryDirectory(prefix="howlframe-bench-")
    built = Path(td.name) / "howlframe"
    subprocess.run(["go", "build", "-o", str(built), "howlframe.go"], cwd=ROOT, check=True)
    return str(built), td


def verify_howlframe(bin_path: str) -> list[str]:
    errors: list[str] = []

    # Task A: compile only (HTTP server).
    a = BENCH / "task_a_hello_json/howlframe/hello.howl"
    with tempfile.TemporaryDirectory() as td:
        bc_a = Path(td) / "a.hfbc"
        r = subprocess.run([bin_path, "-compile-bc", str(a), "-o", str(bc_a)],
                           capture_output=True, text=True)
        if r.returncode != 0:
            errors.append(f"task A compile failed: {r.stderr or r.stdout}")

    # Task B: compile + run with fixture file and missing-file path.
    b = BENCH / "task_b_cli_file_sum/howlframe/greet.howl"
    with tempfile.TemporaryDirectory() as td:
        td_path = Path(td)
        names = td_path / "names.txt"
        names.write_text("Alice\nBob\n\nCarol\n")
        bc = td_path / "greet.hfbc"
        r = subprocess.run([bin_path, "-compile-bc", str(b), "-o", str(bc)],
                           capture_output=True, text=True)
        if r.returncode != 0:
            errors.append(f"task B compile failed: {r.stderr or r.stdout}")
        else:
            old = os.getcwd()
            try:
                os.chdir(td)
                r1 = subprocess.run([bin_path, "-run-bc", "-allow-caps", "filesystem", str(bc)],
                                    capture_output=True, text=True)
            finally:
                os.chdir(old)
            if r1.returncode != 0 or "Hello, Alice" not in r1.stdout:
                errors.append(f"task B run failed: out={r1.stdout!r} err={r1.stderr!r}")
            missing_dir = td_path / "empty"
            missing_dir.mkdir()
            r2 = subprocess.run([bin_path, "-run-bc", "-allow-caps", "filesystem", str(bc)],
                                cwd=missing_dir, capture_output=True, text=True)
            if "Error: could not read names.txt" not in r2.stdout and \
               "Error: could not read names.txt" not in r2.stderr:
                errors.append(f"task B missing-file failed: out={r2.stdout!r} err={r2.stderr!r}")

    # Task C: compile + run (assert path).
    c = BENCH / "task_c_function_test/howlframe/add.howl"
    with tempfile.TemporaryDirectory() as td:
        bc = Path(td) / "add.hfbc"
        r = subprocess.run([bin_path, "-compile-bc", str(c), "-o", str(bc)],
                           capture_output=True, text=True)
        if r.returncode != 0:
            errors.append(f"task C compile failed: {r.stderr or r.stdout}")
        else:
            r1 = subprocess.run([bin_path, "-run-bc", str(bc)], capture_output=True, text=True)
            if r1.returncode != 0:
                errors.append(f"task C run failed: {r1.stderr or r1.stdout}")

    return errors


def recount() -> dict[str, dict[str, int]]:
    out: dict[str, dict[str, int]] = {}
    for task, langs in TASKS.items():
        out[task] = {}
        for lang, rels in langs.items():
            total = 0
            for rel in rels:
                total += count_tokens((BENCH / task / rel).read_text())
            out[task][lang] = total
    return out


def main() -> int:
    args = parse_args()
    bin_path, tmp_bin_dir = howlframe_bin()
    try:
        errors = verify_howlframe(bin_path)
    finally:
        if tmp_bin_dir is not None:
            tmp_bin_dir.cleanup()
    counts = recount()

    print(f"Token method: {METHOD}")
    print(f"{'lang':10} {'A':>5} {'B':>5} {'C':>5} {'tot':>6}")
    totals: dict[str, int] = {lang: 0 for lang in LANG_ORDER}
    for lang in LANG_ORDER:
        a = counts["task_a_hello_json"][lang]
        b = counts["task_b_cli_file_sum"][lang]
        c = counts["task_c_function_test"][lang]
        totals[lang] = a + b + c
        print(f"{lang:10} {a:5} {b:5} {c:5} {totals[lang]:6}")

    # Adoption gates.
    for task, label in [
        ("task_a_hello_json", "A"),
        ("task_b_cli_file_sum", "B"),
        ("task_c_function_test", "C"),
    ]:
        hf = counts[task]["HowlFrame"]
        py = counts[task]["python"]
        if hf >= py:
            errors.append(f"Task {label}: HowlFrame {hf} tokens not under Python {py}")

    for lang in LANG_ORDER:
        if lang == "HowlFrame":
            continue
        if totals["HowlFrame"] >= totals[lang]:
            errors.append(
                f"Total: HowlFrame {totals['HowlFrame']} not under {lang} {totals[lang]}"
            )

    stamp = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    proof = {
        "timestamp": stamp,
        "token_method": METHOD,
        "per_task": counts,
        "totals": totals,
        "verify_errors": errors,
        "passed": len(errors) == 0,
    }
    if args.write:
        out_json = BENCH / "proof_latest.json"
        out_json.write_text(json.dumps(proof, indent=2) + "\n")
        print(f"Wrote {out_json}")

    if errors:
        print("\nFAILED:")
        for e in errors:
            print(f"  - {e}")
        return 1
    print("\nPASSED: HowlFrame under Python on A/B/C and under all language totals.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
