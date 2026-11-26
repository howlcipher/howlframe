import os
import subprocess
import tempfile
from pathlib import Path

from tools.orchestrator.orchestrator import (
    REPO_ROOT,
    extract_howl_source,
    run_repair_loop,
    verify_and_run_howl,
)


def _build_temp_binary(tmp_path: Path) -> list[str]:
    binary = tmp_path / "howlframe"
    subprocess.run(
        ["go", "build", "-o", str(binary), "howlframe.go"],
        cwd=REPO_ROOT,
        check=True,
    )
    return [str(binary)]


def test_extract_howl_source_strips_fences():
    fenced = "```howl\n(cli_app (print 30))\n```"
    assert extract_howl_source(fenced) == "(cli_app (print 30))"
    assert extract_howl_source("  (cli_app (print 30))  ") == "(cli_app (print 30))"


def test_verify_and_run_howl_repair_loop(tmp_path: Path):
    base_cmd = _build_temp_binary(tmp_path)
    prompts: list[str] = []
    attempts = [
        "```howl\n(cli_app (print missing_var))\n```",
        "(cli_app (let (x (+ 10 20)) (print x)))",
    ]

    def fake_model(prompt: str) -> str:
        prompts.append(prompt)
        return attempts[len(prompts) - 1]

    ok, source, output = run_repair_loop(
        fake_model,
        "system\n",
        "Print 10 + 20",
        max_retries=3,
        repo_root=REPO_ROOT,
        base_cmd=base_cmd,
    )

    assert ok is True
    assert len(prompts) == 2
    assert "Verification/execution error" in prompts[1]
    assert "missing_var" in prompts[1]
    assert source == "(cli_app (let (x (+ 10 20)) (print x)))"
    assert output.strip() == "30"


def test_verify_and_run_howl_does_not_dirty_repo_root(tmp_path: Path):
    base_cmd = _build_temp_binary(tmp_path)
    ok, output, diag = verify_and_run_howl(
        "(cli_app (print 42))",
        repo_root=REPO_ROOT,
        base_cmd=base_cmd,
    )
    assert ok is True
    assert output.strip() == "42"
    assert diag["ok"] is True
    assert not (REPO_ROOT / "app.json").exists()
