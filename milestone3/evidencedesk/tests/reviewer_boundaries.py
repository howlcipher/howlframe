"""
Additional evaluator boundaries, uses native independently built app artifacts.
"""

import json
import os
from pathlib import Path
import subprocess
import tempfile

APP = Path(__file__).resolve().parents[1]
results = {}
with tempfile.TemporaryDirectory(prefix="m3-review-") as directory:
    output = Path(directory) / "out"
    output.mkdir()
    result = subprocess.run(
        [
            str(APP / "scripts/project.sh"),
            str(APP / "tests/fixtures/review.jsonl"),
            str(output),
            "00003",
        ],
        env=dict(os.environ, HOWLFRAME_BIN="/tmp/m1-independent-hf"),
        text=True,
        capture_output=True,
    )
    results["zero_padded_positive_MAX"] = {
        "exit": result.returncode,
        "stdout": result.stdout,
        "stderr": result.stderr,
        "files": sorted(p.name for p in output.iterdir()),
        "expected_exit": 0,
    }
print(json.dumps(results, indent=2))
