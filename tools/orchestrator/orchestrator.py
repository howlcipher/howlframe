"""Optional local-model orchestrator driving `.howl` synthesis, check, build, and run."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any, Callable

SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent.parent

_CODE_FENCE_RE = re.compile(
    r"^\s*```(?:howl|lisp|scheme|text)?\s*\n(?P<body>.*?)\n```\s*$",
    re.DOTALL | re.IGNORECASE,
)


def extract_howl_source(raw: str) -> str:
    """Normalize model output into raw HowlFrame S-expression source."""
    text = raw.strip()
    match = _CODE_FENCE_RE.match(text)
    if match:
        return match.group("body").strip()
    return text


def resolve_howlframe_cmd(repo_root: Path = REPO_ROOT) -> list[str]:
    """Return an argv prefix pointing to the repository HowlFrame binary or source."""
    repo_bin = repo_root / "howlframe"
    if repo_bin.is_file() and os.access(repo_bin, os.X_OK):
        return [str(repo_bin)]
    which_bin = shutil.which("howlframe")
    if which_bin:
        return [which_bin]
    return ["go", "run", "howlframe.go"]


def _parse_json_output(stdout: str, stderr: str) -> dict[str, Any]:
    for stream in (stdout, stderr):
        text = stream.strip()
        if not text:
            continue
        try:
            parsed = json.loads(text)
            if isinstance(parsed, dict):
                return parsed
        except json.JSONDecodeError:
            pass
    return {}


def verify_and_run_howl(
    source: str,
    repo_root: Path = REPO_ROOT,
    allow_caps: str = "",
    base_cmd: list[str] | None = None,
    timeout: int = 30,
) -> tuple[bool, str, dict[str, Any]]:
    """Verify `.howl` source with `agent check --json`, build HFBC, and run it."""
    cmd_prefix = list(base_cmd) if base_cmd is not None else resolve_howlframe_cmd(repo_root)
    cleaned = extract_howl_source(source)
    if not cleaned:
        return False, "empty HowlFrame source", {"stage": "extract", "reason": "empty source"}

    with tempfile.TemporaryDirectory(prefix="howlframe-orch-") as tmp:
        tmp_dir = Path(tmp)
        src_path = tmp_dir / "app.howl"
        artifact_path = tmp_dir / "app.hfbc"
        src_path.write_text(cleaned + "\n", encoding="utf-8")

        check_proc = subprocess.run(
            [*cmd_prefix, "agent", "check", "--json", str(src_path)],
            cwd=repo_root,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
        check_info = _parse_json_output(check_proc.stdout, check_proc.stderr)
        if check_proc.returncode != 0 or check_info.get("ok") is False:
            msg = check_proc.stdout.strip() or check_proc.stderr.strip() or "howlframe agent check failed"
            return False, msg, {"stage": "check", **check_info}

        build_proc = subprocess.run(
            [*cmd_prefix, "build", "--target", "bytecode", "-o", str(artifact_path), str(src_path)],
            cwd=repo_root,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
        if build_proc.returncode != 0:
            err_info = _parse_json_output(build_proc.stdout, build_proc.stderr)
            msg = build_proc.stdout.strip() or build_proc.stderr.strip() or "howlframe build failed"
            return False, msg, {"stage": "build", **err_info}

        run_cmd = [*cmd_prefix, "run"]
        if allow_caps:
            run_cmd.extend(["--allow-caps", allow_caps])
        run_cmd.append(str(artifact_path))

        run_proc = subprocess.run(
            run_cmd,
            cwd=tmp_dir,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
        if run_proc.returncode != 0:
            err_info = _parse_json_output(run_proc.stdout, run_proc.stderr)
            msg = run_proc.stderr.strip() or run_proc.stdout.strip() or f"exit status {run_proc.returncode}"
            return False, msg, {"stage": "run", **err_info}

        return True, run_proc.stdout, {"stage": "run", "ok": True, "stdout": run_proc.stdout}


def run_repair_loop(
    generate_fn: Callable[[str], str],
    system_context: str,
    user_goal: str,
    *,
    max_retries: int = 3,
    repo_root: Path = REPO_ROOT,
    allow_caps: str = "",
    base_cmd: list[str] | None = None,
) -> tuple[bool, str, str]:
    """Generate, check, build, and execute `.howl` code with structured feedback."""
    base_prompt = f"{system_context}Goal:\n{user_goal}\nReturn only valid HowlFrame (.howl) S-expression source."
    current_prompt = base_prompt
    last_output = ""
    last_source = ""

    for _ in range(max_retries):
        candidate = generate_fn(current_prompt)
        last_source = extract_howl_source(candidate)
        ok, output, diagnostics = verify_and_run_howl(
            last_source,
            repo_root=repo_root,
            allow_caps=allow_caps,
            base_cmd=base_cmd,
        )
        last_output = output
        if ok:
            return True, last_source, output

        feedback = json.dumps(diagnostics) if diagnostics else output
        current_prompt = (
            f"{base_prompt}\n\nPrevious attempt:\n{last_source}\n\n"
            f"Verification/execution error:\n{feedback}\n"
            "Fix the .howl source and output only the corrected S-expression."
        )

    return False, last_source, last_output


def load_system_context() -> str:
    prompt_file = SCRIPT_DIR / "ai_prompt.md"
    if prompt_file.is_file():
        return prompt_file.read_text(encoding="utf-8") + "\n\n"
    return ""


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--goal",
        default="Calculate the sum of 10 and 20, store it in a variable, and print it.",
        help="Natural-language goal for the .howl program",
    )
    parser.add_argument("--model", default="llama3", help="Local Ollama model name")
    parser.add_argument("--base-url", default="http://localhost:11434/v1", help="OpenAI-compatible endpoint URL")
    parser.add_argument("--max-retries", type=int, default=3, help="Maximum generate-check-run iterations")
    parser.add_argument("--allow-caps", default="", help="Capabilities passed to `howlframe run`")
    args = parser.parse_args()

    try:
        from openai import OpenAI
    except ImportError as exc:
        print(f"Failed to import openai client: {exc}", file=sys.stderr)
        return 1

    client = OpenAI(base_url=args.base_url, api_key="ollama")

    def generate_with_ollama(prompt: str) -> str:
        resp = client.chat.completions.create(
            model=args.model,
            messages=[{"role": "user", "content": prompt}],
            temperature=0.0,
        )
        return resp.choices[0].message.content or ""

    ok, source, output = run_repair_loop(
        generate_with_ollama,
        load_system_context(),
        args.goal,
        max_retries=args.max_retries,
        allow_caps=args.allow_caps,
    )
    if ok:
        print("Verified and executed HowlFrame source:\n")
        print(source)
        print("\nOutput:")
        print(output)
        return 0

    print("Failed after maximum retries.", file=sys.stderr)
    print(output, file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
