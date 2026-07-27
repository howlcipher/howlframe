import argparse
import json
import os
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Iterable, Sequence

from openai import OpenAI

COMMAND_TIMEOUT_SECONDS = 300
PATCH_POLL_SECONDS = 1.0
TRANSIENT_NAMES = {
    ".git",
    ".pytest_cache",
    "__pycache__",
    "crash.json",
    "server.go",
    "server_test.go",
    "telemetry.jsonl",
}


@dataclass(frozen=True)
class PatchResult:
    status: str


def resolve_project_path(
    project_root: Path,
    candidate_path: Path,
) -> Path:
    """Resolve a path while constraining it to the project root."""
    resolved_root = project_root.resolve(strict=True)
    if candidate_path.is_absolute():
        resolved_candidate = candidate_path.resolve()
    else:
        resolved_candidate = (resolved_root / candidate_path).resolve()
    try:
        resolved_candidate.relative_to(resolved_root)
    except ValueError as exc:
        raise ValueError(
            "candidate path must be inside project root"
        ) from exc
    return resolved_candidate


def _load_crash(crash_path: Path) -> dict:
    crash_data = json.loads(crash_path.read_text(encoding="utf-8"))
    if not isinstance(crash_data, dict):
        raise ValueError("crash dump must be a JSON object")
    return crash_data


def _candidate_from_response(content: str) -> str:
    response_data = json.loads(content)
    if (
        not isinstance(response_data, dict)
        or set(response_data) != {"source"}
    ):
        raise ValueError("model response must contain only source")
    source = response_data["source"]
    if not isinstance(source, str) or not source.strip():
        raise ValueError("model source must be a non-empty string")
    return source


def _copy_ignore(_directory: str, names: list[str]) -> set[str]:
    return set(names).intersection(TRANSIENT_NAMES)


def _model_prompt(source: str, crash_data: dict) -> str:
    crash_json = json.dumps(
        crash_data,
        ensure_ascii=True,
        separators=(",", ":"),
    )
    return (
        "Repair this Zero source using the crash dump. Return strict JSON "
        'with exactly one key named "source" containing the complete '
        "replacement. Do not return commands, paths, markdown, or commentary."
        f"\n\nCrash dump:\n{crash_json}\n\nCurrent source:\n{source}"
    )


def _run_command(
    runner: Callable[..., subprocess.CompletedProcess],
    command: Sequence[str],
    cwd: Path,
) -> subprocess.CompletedProcess:
    return runner(
        tuple(command),
        cwd=cwd,
        shell=False,
        text=True,
        capture_output=True,
        timeout=COMMAND_TIMEOUT_SECONDS,
    )


def _install_atomically(source_path: Path, candidate_source: str) -> None:
    original_mode = source_path.stat().st_mode
    file_descriptor, temporary_name = tempfile.mkstemp(
        dir=source_path.parent,
        prefix=f".{source_path.name}.",
    )
    temporary_path = Path(temporary_name)
    try:
        with os.fdopen(file_descriptor, "w", encoding="utf-8") as stream:
            stream.write(candidate_source)
            stream.flush()
            os.fsync(stream.fileno())
        os.chmod(temporary_path, original_mode)
        os.replace(temporary_path, source_path)
    finally:
        temporary_path.unlink(missing_ok=True)


def apply_crash_patch(
    *,
    client: object,
    project_root: Path,
    source_path: Path,
    crash_path: Path,
    test_command: Sequence[str],
    restart_command: Sequence[str],
    runner: Callable[..., subprocess.CompletedProcess] = subprocess.run,
) -> PatchResult:
    """Verify and install one model-proposed Zero source replacement."""
    try:
        resolved_root = project_root.resolve(strict=True)
        resolved_source = resolve_project_path(
            resolved_root,
            source_path,
        )
        resolved_crash = resolve_project_path(
            resolved_root,
            crash_path,
        )
        if resolved_source.suffix != ".zero":
            raise ValueError("source must use the .zero extension")
        current_source = resolved_source.read_text(encoding="utf-8")
        crash_data = _load_crash(resolved_crash)
        response = client.chat.completions.create(
            model="llama3",
            messages=[
                {
                    "role": "user",
                    "content": _model_prompt(current_source, crash_data),
                }
            ],
            response_format={"type": "json_object"},
        )
        candidate_source = _candidate_from_response(
            response.choices[0].message.content
        )
    except Exception:
        return PatchResult("rejected")

    try:
        with tempfile.TemporaryDirectory(
            prefix="zero-patch-"
        ) as temporary_directory:
            isolated_root = Path(temporary_directory)
            shutil.copytree(
                resolved_root,
                isolated_root,
                dirs_exist_ok=True,
                ignore=_copy_ignore,
            )
            relative_source = resolved_source.relative_to(resolved_root)
            isolated_source = isolated_root / relative_source
            isolated_source.write_text(
                candidate_source,
                encoding="utf-8",
            )
            isolated_command = tuple(
                str(isolated_source) if part == "{source}" else part
                for part in test_command
            )
            test_result = _run_command(
                runner,
                isolated_command,
                isolated_root,
            )
            if test_result.returncode != 0:
                return PatchResult("test_failed")
    except Exception:
        return PatchResult("test_failed")

    try:
        _install_atomically(resolved_source, candidate_source)
    except Exception:
        return PatchResult("install_failed")

    try:
        restart_result = _run_command(
            runner,
            restart_command,
            resolved_root,
        )
    except Exception:
        return PatchResult("restart_failed")
    if restart_result.returncode != 0:
        return PatchResult("restart_failed")
    return PatchResult("applied")


def _log_event(event: str, status: str) -> None:
    print(
        json.dumps(
            {
                "event": event,
                "status": status,
                "timestamp": int(time.time()),
            },
            separators=(",", ":"),
        ),
        flush=True,
    )


def watch_crashes(
    *,
    client: object,
    project_root: Path,
    source_path: Path,
    crash_path: Path,
    test_command: Sequence[str],
    restart_command: Sequence[str],
    once: bool = False,
) -> None:
    """Process each new crash dump once."""
    previous_marker: tuple[int, int] | None = None
    while True:
        try:
            stat_result = crash_path.stat()
            marker = (
                stat_result.st_mtime_ns,
                stat_result.st_size,
            )
        except OSError:
            marker = None
        if marker is not None and marker != previous_marker:
            previous_marker = marker
            result = apply_crash_patch(
                client=client,
                project_root=project_root,
                source_path=source_path,
                crash_path=crash_path,
                test_command=test_command,
                restart_command=restart_command,
            )
            _log_event("crash_patch", result.status)
            if once:
                return
        elif once:
            _log_event("crash_patch", "no_crash")
            return
        time.sleep(PATCH_POLL_SECONDS)


def tail_file(filepath: Path):
    """Yield telemetry lines as they are appended."""
    with filepath.open("r", encoding="utf-8") as stream:
        stream.seek(0, os.SEEK_END)
        while True:
            line = stream.readline()
            if not line:
                time.sleep(0.1)
                continue
            yield line


def check_anomalies(client: object, events: Iterable[str]) -> None:
    prompt = (
        "Review this Go application telemetry for anomalies, infinite loops, "
        "or unexpected state changes. Respond NORMAL when no anomaly exists."
        "\n\nEvents:\n"
        + "\n".join(events)
    )
    try:
        response = client.chat.completions.create(
            model="llama3",
            messages=[{"role": "user", "content": prompt}],
            max_tokens=200,
        )
        reply = response.choices[0].message.content.strip()
        if "NORMAL" not in reply.upper() or len(reply) > 20:
            _log_event("telemetry_analysis", "anomaly")
        else:
            _log_event("telemetry_analysis", "normal")
    except Exception:
        _log_event("telemetry_analysis", "request_failed")


def observe_telemetry(client: object, telemetry_file: Path) -> None:
    telemetry_file.touch(exist_ok=True)
    events: list[str] = []
    try:
        for line in tail_file(telemetry_file):
            stripped_line = line.strip()
            if stripped_line:
                events.append(stripped_line)
            if len(events) >= 10:
                check_anomalies(client, events)
                events = []
    except KeyboardInterrupt:
        _log_event("observer", "stopped")


def _command(value: str) -> tuple[str, ...]:
    command = tuple(shlex.split(value))
    if not command:
        raise argparse.ArgumentTypeError("command cannot be empty")
    return command


def _argument_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Zero observer")
    subparsers = parser.add_subparsers(dest="mode")
    subparsers.add_parser("observe", help="watch telemetry")
    patch_parser = subparsers.add_parser(
        "patch",
        help="watch crash dumps and test bounded source replacements",
    )
    patch_parser.add_argument(
        "--project-root",
        required=True,
        type=Path,
    )
    patch_parser.add_argument("--source", required=True, type=Path)
    patch_parser.add_argument(
        "--crash-file",
        default=Path("crash.json"),
        type=Path,
    )
    patch_parser.add_argument(
        "--test-command",
        required=True,
        type=_command,
    )
    patch_parser.add_argument(
        "--restart-command",
        required=True,
        type=_command,
    )
    patch_parser.add_argument(
        "--once",
        action="store_true",
        help="process the current crash once and exit",
    )
    return parser


def main() -> int:
    arguments = _argument_parser().parse_args()
    client = OpenAI(
        base_url="http://localhost:11434/v1",
        api_key="ollama",
    )
    if arguments.mode == "patch":
        try:
            project_root = arguments.project_root.resolve(strict=True)
            source_path = resolve_project_path(
                project_root,
                arguments.source,
            )
            crash_path = resolve_project_path(
                project_root,
                arguments.crash_file,
            )
        except (OSError, ValueError):
            _log_event("crash_patch", "invalid_configuration")
            return 2
        watch_crashes(
            client=client,
            project_root=project_root,
            source_path=source_path,
            crash_path=crash_path,
            test_command=arguments.test_command,
            restart_command=arguments.restart_command,
            once=arguments.once,
        )
        return 0
    observe_telemetry(client, Path("telemetry.jsonl"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
