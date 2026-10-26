"""Blind-grading packets (protocol §6 diagnostic quality, §4 delegation flags).

    python3 -m cv1.blind RUNS_ROOT --out PACKET_DIR [--seed N]

Writes `items.json` for graders and `key.json` for the analyst, which graders must not see. Each item gets a random
code, and the arm name, file extensions, toolchain names and paths are masked where possible. Two kinds of item:

* `diagnostic`: the first compiler or runtime error the agent hit (from `cv1.transcript`), to score with the frozen
  1–5 rubric;
* `delegation`: a solution the scorer flagged as possibly handing its work to another language, with the flagged
  files, for a yes or no ruling.

Limits: masking is lexical. Source code shown for a delegation ruling necessarily reveals its language to a reader who
knows the syntax, and an error message's wording can still hint at its toolchain. Tool-call wrappers are removed so the
provider's CLI conventions are not shown, but graders should be told that the masking is best effort.
"""

import argparse
import glob
import json
import os
import random
import re
import secrets
import sys

from . import transcript

MASKS = [
    (re.compile(r"\b(howlframe|howl|python3?|pydoc|golang|go(?= (build|run|vet|doc|test|mod))|node(js)?|npm|"
                r"javascript|typescript|perl|ruby|php|lua)\b", re.I), "<tool>"),
    (re.compile(r"\.(howl|hfbc|py|pyc|go|js|mjs|cjs|ts|pl|rb|php|lua|awk|c|cc|cpp|rs|java)\b"), ".<ext>"),
    (re.compile(r"/opt/(toolchain|docs)\S*"), "<path>"),
    (re.compile(r"\b(Traceback \(most recent call last\)|panic:|goroutine \d+)"), "<error-kind>"),
    (re.compile(r"\b[A-Z][A-Za-z]*(Error|Exception)\b"), "<error-kind>"),
    (re.compile(r'File "[^"]*", line (\d+)(, in \S+)?'), r"<file> line \1"),
    (re.compile(r"^\s*at .*$", re.M), "  <frame>"),
    (re.compile(r"(?:\b(?:Exit code:?|Process exited with code|exited with (?:code|status))|\"exit_code\"\s*:)"
                r"\s*(-?\d+)"), r"<exit \1>"),
    (re.compile(r"\b(Script completed|Wall time [\d.]+ seconds|Output:|chunk_id|wall_time_seconds|"
                r"original_token_count)\b"), ""),
]


def command_text(raw):
    """The command an agent ran, without the provider's tool-call wrapper (JSON input or code-mode script)."""
    try:
        data = json.loads(raw)
    except (TypeError, ValueError):
        data = None
    if isinstance(data, dict):
        for key in ("command", "cmd", "file_path", "path"):
            if isinstance(data.get(key), str):
                return data[key]
    m = re.search(r"\bcmd\s*:\s*(\"(?:[^\"\\]|\\.)*\")", raw or "")
    if m:
        try:
            return json.loads(m.group(1))
        except ValueError:
            pass
    return raw or ""


def mask(text):
    for pattern, repl in MASKS:
        text = pattern.sub(repl, text)
    return text


def build(root, seed=None):
    rng = random.Random(seed if seed is not None else secrets.randbits(64))
    items, key = [], {}
    for result_path in sorted(glob.glob(os.path.join(root, "**", "result.json"), recursive=True)):
        run_dir = os.path.dirname(result_path)
        with open(result_path) as f:
            result = json.load(f)
        meta = {"run_id": result["run_id"], "task": result["task_id"], "arm": result["arm"],
                "provider": result["agent"]["provider"]}
        summary = transcript.summarize(run_dir)
        if summary["first_error"]:
            fe = summary["first_error"]
            items.append(({"kind": "diagnostic", "command": mask(command_text(fe["command"])),
                           "output": mask(fe["output"])}, meta))
        score_path = os.path.join(run_dir, "score.json")
        if os.path.exists(score_path):
            with open(score_path) as f:
                flags = json.load(f).get("delegation_flags") or []
            if flags:
                files = {}
                for fl in flags:
                    path = os.path.join(run_dir, "workspace", fl["file"])
                    if fl["file"] not in files and os.path.isfile(path):
                        with open(path, errors="replace") as fh:
                            files[fl["file"]] = mask(fh.read()[:20000])
                masked = [{k: mask(str(v)) for k, v in fl.items()} for fl in flags]
                items.append(({"kind": "delegation", "flags": masked,
                               "files": {mask(k): v for k, v in files.items()}}, meta))
    rng.shuffle(items)
    out = []
    for item, meta in items:
        code = f"{rng.getrandbits(32):08x}"
        out.append(dict(item, code=code))
        key[code] = dict(meta, kind=item["kind"])
    return out, key


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("root")
    p.add_argument("--out", required=True)
    p.add_argument("--seed", type=int)
    args = p.parse_args(argv)
    items, key = build(args.root, args.seed)
    os.makedirs(args.out, exist_ok=True)
    with open(os.path.join(args.out, "items.json"), "w") as f:
        json.dump(items, f, indent=2)
    with open(os.path.join(args.out, "key.json"), "w") as f:
        json.dump(key, f, indent=2)
    print(json.dumps({"items": len(items), "diagnostic": sum(i["kind"] == "diagnostic" for i in items),
                      "delegation": sum(i["kind"] == "delegation" for i in items)}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
