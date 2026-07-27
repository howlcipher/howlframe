import json
import subprocess
from pathlib import Path
from unittest.mock import Mock

import pytest

import observer


def write_fixture(project_root: Path) -> tuple[Path, Path]:
    source_path = project_root / "app.zero"
    source_path.write_text('(cli_app (print "old"))\n', encoding="utf-8")
    crash_path = project_root / "crash.json"
    crash_path.write_text(
        json.dumps({"panic": "boom", "stack": "main.main"}),
        encoding="utf-8",
    )
    return source_path, crash_path


def model_client(candidate: str) -> Mock:
    client = Mock()
    choice = Mock()
    choice.message.content = json.dumps({"source": candidate})
    client.chat.completions.create.return_value.choices = [choice]
    return client


def test_resolve_project_path_rejects_escape(tmp_path):
    outside = tmp_path.parent / "outside.zero"

    with pytest.raises(ValueError, match="inside project root"):
        observer.resolve_project_path(tmp_path, outside)


def test_malformed_model_output_does_not_change_source(tmp_path):
    source_path, crash_path = write_fixture(tmp_path)
    original = source_path.read_text(encoding="utf-8")
    client = model_client("unused")
    client.chat.completions.create.return_value.choices[
        0
    ].message.content = "not json"
    runner = Mock()

    result = observer.apply_crash_patch(
        client=client,
        project_root=tmp_path,
        source_path=source_path,
        crash_path=crash_path,
        test_command=("python", "-c", "raise SystemExit(0)"),
        restart_command=("service", "restart"),
        runner=runner,
    )

    assert result.status == "rejected"
    assert source_path.read_text(encoding="utf-8") == original
    runner.assert_not_called()


def test_failed_candidate_tests_preserve_source_and_skip_restart(tmp_path):
    source_path, crash_path = write_fixture(tmp_path)
    original = source_path.read_text(encoding="utf-8")
    candidate = '(cli_app (print "candidate"))\n'
    client = model_client(candidate)
    runner = Mock(
        return_value=subprocess.CompletedProcess(
            args=[],
            returncode=1,
            stdout="",
            stderr="tests failed",
        )
    )

    result = observer.apply_crash_patch(
        client=client,
        project_root=tmp_path,
        source_path=source_path,
        crash_path=crash_path,
        test_command=("verify", "{source}"),
        restart_command=("service", "restart"),
        runner=runner,
    )

    assert result.status == "test_failed"
    assert source_path.read_text(encoding="utf-8") == original
    assert runner.call_count == 1
    assert runner.call_args.args[0][0] == "verify"
    assert Path(runner.call_args.args[0][1]).name == "app.zero"
    assert runner.call_args.kwargs["shell"] is False


def test_passing_candidate_is_installed_then_restarted(tmp_path):
    source_path, crash_path = write_fixture(tmp_path)
    candidate = '(cli_app (print "fixed"))\n'
    client = model_client(candidate)
    runner = Mock(
        side_effect=[
            subprocess.CompletedProcess([], 0, "tests passed", ""),
            subprocess.CompletedProcess([], 0, "restarted", ""),
        ]
    )

    result = observer.apply_crash_patch(
        client=client,
        project_root=tmp_path,
        source_path=source_path,
        crash_path=crash_path,
        test_command=("verify", "{source}"),
        restart_command=("service", "restart"),
        runner=runner,
    )

    assert result.status == "applied"
    assert source_path.read_text(encoding="utf-8") == candidate
    assert runner.call_count == 2
    assert runner.call_args_list[1].args[0] == ("service", "restart")
    assert all(
        call.kwargs["shell"] is False for call in runner.call_args_list
    )
