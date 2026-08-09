import json
import subprocess
import sys
from pathlib import Path
from unittest.mock import Mock

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from tools.observer_agent import observer_agent as observer  # noqa: E402


def write_fixture(project_root: Path) -> tuple[Path, Path]:
    source_path = project_root / "app.howl"
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
    outside = tmp_path.parent / "outside.howl"

    with pytest.raises(ValueError, match="inside project root"):
        observer.resolve_project_path(tmp_path, outside)


def test_resolve_project_path_resolves_relative_to_root(tmp_path, monkeypatch):
    source_path, _ = write_fixture(tmp_path)
    monkeypatch.chdir(tmp_path.parent)

    resolved = observer.resolve_project_path(tmp_path, Path("app.howl"))

    assert resolved == source_path


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
    assert Path(runner.call_args.args[0][1]).name == "app.howl"
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
    prompt = client.chat.completions.create.call_args.kwargs["messages"][0][
        "content"
    ]
    assert '"panic"' in prompt
    assert '"boom"' in prompt
    assert '(cli_app (print "old"))' in prompt


def test_non_howlframe_source_is_rejected(tmp_path):
    source_path, crash_path = write_fixture(tmp_path)
    source_path = source_path.with_suffix(".txt")
    source_path.write_text("not HowlFrame source", encoding="utf-8")

    result = observer.apply_crash_patch(
        client=model_client("candidate"),
        project_root=tmp_path,
        source_path=source_path,
        crash_path=crash_path,
        test_command=("verify", "{source}"),
        restart_command=("service", "restart"),
        runner=Mock(),
    )

    assert result.status == "rejected"


def test_model_failure_is_rejected_without_running_commands(tmp_path):
    source_path, crash_path = write_fixture(tmp_path)
    client = Mock()
    client.chat.completions.create.side_effect = RuntimeError("offline")
    runner = Mock()

    result = observer.apply_crash_patch(
        client=client,
        project_root=tmp_path,
        source_path=source_path,
        crash_path=crash_path,
        test_command=("verify", "{source}"),
        restart_command=("service", "restart"),
        runner=runner,
    )

    assert result.status == "rejected"
    runner.assert_not_called()


def test_real_commands_validate_copy_and_restart_live_project(tmp_path):
    source_path, crash_path = write_fixture(tmp_path)
    candidate = '(cli_app (print "fixed"))\n'
    restart_marker = tmp_path / "restarted.txt"
    verify_script = (
        "from pathlib import Path; import sys; "
        "source = Path(sys.argv[1]); "
        "raise SystemExit(0 if 'fixed' in source.read_text() else 1)"
    )
    restart_script = (
        "from pathlib import Path; "
        "Path('restarted.txt').write_text('yes', encoding='utf-8')"
    )

    result = observer.apply_crash_patch(
        client=model_client(candidate),
        project_root=tmp_path,
        source_path=source_path,
        crash_path=crash_path,
        test_command=(sys.executable, "-c", verify_script, "{source}"),
        restart_command=(sys.executable, "-c", restart_script),
        runner=subprocess.run,
    )

    assert result.status == "applied"
    assert source_path.read_text(encoding="utf-8") == candidate
    assert restart_marker.read_text(encoding="utf-8") == "yes"


def test_observability_layer_tracks_telemetry_and_anomalies():
    layer = observer.ObservabilityLayer(capacity=2)
    layer.add_telemetry('{"id": 1}')
    layer.add_telemetry('{"id": 2}')
    layer.add_telemetry('{"id": 3}')

    view = layer.get_view()
    assert len(view["recent_telemetry"]) == 2
    assert view["recent_telemetry"][0] == {"id": 2}
    assert view["recent_telemetry"][1] == {"id": 3}
    assert view["status"] == "normal"
    assert view["anomalies_detected"] == 0

    layer.report_anomaly(123456789)
    view = layer.get_view()
    assert view["status"] == "anomaly"
    assert view["anomalies_detected"] == 1
    assert view["last_anomaly_time"] == 123456789

    layer.report_normal()
    view = layer.get_view()
    assert view["status"] == "normal"
    assert view["anomalies_detected"] == 1
