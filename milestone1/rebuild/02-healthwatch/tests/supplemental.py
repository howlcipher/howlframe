"""Small additional input-boundary checks; no frozen test modification."""

import json
import os
from pathlib import Path
import subprocess
import tempfile

HF = os.environ.get("HF_BIN", "/tmp/healthwatch-hf")
ARTIFACT = os.environ.get("APP_ARTIFACT", "/tmp/healthwatch-fresh.hfbc")


def invoke(args, text="", grants=""):
    command = [HF, "run"]
    if grants:
        command += ["--allow-caps", grants]
    return subprocess.run(
        command + [ARTIFACT] + args,
        input=text,
        text=True,
        capture_output=True,
        timeout=10,
    )


def main():
    cases = [
        ({"version": 1, "checks": [{"name": "a", "url": "http://h"}]}, 0),
        ({"version": True, "checks": []}, 65),
        (
            {
                "version": 1,
                "checks": [
                    {"name": "a", "url": "http://h", "expect_status": True}
                ],
            },
            65,
        ),
        (
            {
                "version": 1,
                "checks": [
                    {
                        "name": "a",
                        "url": "http://h",
                        "json_assert": [{"path": "a", "equals": None}],
                    }
                ],
            },
            0,
        ),
    ]
    for config, expected in cases:
        result = invoke(["validate", "-"], json.dumps(config))
        assert result.returncode == expected, result
    with tempfile.TemporaryDirectory(
        prefix="healthwatch-supplemental-"
    ) as folder:
        state = Path(folder) / "state.json"
        entry = {
            "health": "up",
            "since": 1,
            "last_run": 1,
            "last_status": "PASS",
            "consecutive_failures": 0,
        }
        document = {"version": 1, "checks": {"a": entry}}
        state.write_text(json.dumps(document))
        before = state.read_bytes()
        result = invoke(
            ["status", str(state)], grants=f"filesystem:read={folder}"
        )
        assert result.returncode == 65, result
        assert state.read_bytes() == before
        entry["last_http_status"] = None
        state.write_text(json.dumps(document))
        result = invoke(
            ["status", "--format", "json", str(state)],
            grants=f"filesystem:read={folder}",
        )
        assert result.returncode == 0, result
        assert (
            json.loads(result.stdout)["checks"][0]["last_http_status"] is None
        )
    print("PASS: 6 supplemental input and state boundaries")


if __name__ == "__main__":
    main()
