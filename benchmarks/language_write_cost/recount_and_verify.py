#!/usr/bin/env python3
"""Recount write-cost tokens and verify HowlFrame fixtures beat baselines.

Exit 0 only when:
  - every HowlFrame v1 fixture verifies (compile + run where applicable)
  - HowlFrame tokens < Python tokens on tasks A, B, and C
  - HowlFrame total tokens < Python, Node, Go, C#, and Java totals
"""

from __future__ import annotations

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


def howlframe_bin() -> str:
    candidate = ROOT / "howlframe"
    if candidate.exists():
        return str(candidate)
    built = Path(tempfile.gettempdir()) / "howlframe-bench"
    subprocess.run(["go", "build", "-o", str(built), "howlframe.go"], cwd=ROOT, check=True)
    return str(built)


def verify_howlframe(bin_path: str) -> list[str]:
    errors: list[str] = []

    # Task A: compile only (HTTP server).
    a = BENCH / "task_a_hello_json/howlframe/hello.howl"
    r = subprocess.run([bin_path, "-compile-bc", str(a), "-o", str(BENCH / "_a.hfbc")],
                       capture_output=True, text=True)
    if r.returncode != 0:
        errors.append(f"task A compile failed: {r.stderr or r.stdout}")
    else:
        (BENCH / "_a.hfbc").unlink(missing_ok=True)

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
    bin_path = howlframe_bin()
    errors = verify_howlframe(bin_path)
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
    out_json = BENCH / "proof_latest.json"
    out_json.write_text(json.dumps(proof, indent=2) + "\n")

    if errors:
        print("\nFAILED:")
        for e in errors:
            print(f"  - {e}")
        return 1
    print("\nPASSED: HowlFrame under Python on A/B/C and under all language totals.")
    print(f"Wrote {out_json}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
