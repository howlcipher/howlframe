"""Count verified source bytes with cached tokenizers; no network fallback."""
import csv
import hashlib
import io
import json
from pathlib import Path
import statistics
import tokenize

import tiktoken
import tiktoken.load

ROOT = Path(__file__).resolve().parent
LANGUAGES = {"howl": "howl", "python": "py", "js": "js", "go": "go"}


def offline_read(*args, **kwargs):
    raise RuntimeError("Offline benchmark: tokenizer cache missing; network disabled")


def strip_comments(source, lang):
    """Preserve strings, indentation and nonblank source lines.

    The JS/Go scanner supports the strings in these references (no regex or
    template literals). Python uses tokenize; HowlFrame uses ; line comments.
    """
    if lang == "python":
        lines = source.splitlines(keepends=True)
        for token in tokenize.generate_tokens(io.StringIO(source).readline):
            if token.type == tokenize.COMMENT:
                row, start = token.start
                _, end = token.end
                lines[row - 1] = lines[row - 1][:start] + lines[row - 1][end:]
        cleaned = "".join(lines)
    else:
        out = []
        i = 0
        quote = None
        while i < len(source):
            c = source[i]
            if quote:
                out.append(c)
                i += 1
                if c == "\\" and quote != "`" and i < len(source):
                    out.append(source[i])
                    i += 1
                elif c == quote:
                    quote = None
            elif c in ('"', "'", "`"):
                quote = c
                out.append(c)
                i += 1
            elif (lang == "howl" and c == ";") or (lang != "howl" and source[i:i+2] == "//"):
                end = source.find("\n", i)
                i = len(source) if end == -1 else end
            elif lang != "howl" and source[i:i+2] == "/*":
                end = source.find("*/", i + 2)
                if end == -1:
                    raise ValueError("unterminated comment")
                out.append(" " + "\n" * source[i:end+2].count("\n"))
                i = end + 2
            else:
                out.append(c)
                i += 1
        cleaned = "".join(out)
    return "".join(line for line in cleaned.splitlines(keepends=True) if line.strip())


def summarize(rows):
    summary = {}
    for encoding in ("cl100k_base", "o200k_base"):
        summary[encoding] = {}
        for metric in ("tokens", "tokens_nocomments"):
            by_lang = {lang: {r["task"]: r[metric] for r in rows if r["encoding"] == encoding and r["lang"] == lang} for lang in LANGUAGES}
            stats = {lang: {"median": statistics.median(tasks.values()), "mean": statistics.mean(tasks.values())} for lang, tasks in by_lang.items()}
            ratios = {}
            for lang in ("python", "js", "go"):
                ratios[lang] = {
                    "median_per_task_ratio": statistics.median(by_lang["howl"][task] / n for task, n in by_lang[lang].items()),
                    "ratio_of_medians": stats["howl"]["median"] / stats[lang]["median"],
                    "howl_more_tokens": [task for task, n in by_lang[lang].items() if by_lang["howl"][task] > n],
                }
            summary[encoding][metric] = {"languages": stats, "howl_ratios": ratios}
    return summary


def main():
    if tiktoken.__version__ != "0.8.0":
        raise RuntimeError("Install requirements.txt: tiktoken==0.8.0 required")
    # read_file_cached returns verified cached bytes before calling read_file.
    # Disable the uncached loader so a missing/corrupt cache fails offline.
    tiktoken.load.read_file = offline_read
    encodings = {name: tiktoken.get_encoding(name) for name in ("cl100k_base", "o200k_base")}
    manifest = json.loads((ROOT.parent / "falsifiable-v0/tasks/manifest.json").read_text())
    tasks = [t["id"] for t in manifest["tasks"] + manifest["adversarial"]]
    verification = json.loads((ROOT / "results/verification.json").read_text())
    checks = {(r["task"], r["lang"]): r for r in verification}
    expected = {(task, lang) for task in tasks for lang in LANGUAGES}
    if set(checks) != expected or len(verification) != len(expected):
        raise RuntimeError("Verification coverage must be exactly 60 x 4")
    rows = []
    for task in tasks:
        for lang, ext in LANGUAGES.items():
            raw = (ROOT / f"solutions/{lang}/{task}.{ext}").read_bytes()
            check = checks[task, lang]
            if hashlib.sha256(raw).hexdigest() != check["sha256"]:
                raise RuntimeError(f"Source changed since verification: {task}/{lang}")
            source = raw.decode("utf-8")
            stripped = strip_comments(source, lang)
            for name, encoding in encodings.items():
                rows.append({"task": task, "lang": lang, "encoding": name,
                             "tokens": len(encoding.encode(source, disallowed_special=())),
                             "tokens_nocomments": len(encoding.encode(stripped, disallowed_special=())),
                             "bytes": len(raw), "pass": check["pass"]})
    (ROOT / "results/tokens.json").write_text(json.dumps(rows, indent=2) + "\n")
    with (ROOT / "results/tokens.csv").open("w", newline="") as output:
        writer = csv.DictWriter(output, fieldnames=list(rows[0]))
        writer.writeheader()
        writer.writerows(rows)
    summary = summarize(rows)
    (ROOT / "results/summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps(summary, indent=2))
    for lang in LANGUAGES:
        print(f"{lang}: {sum(checks[task, lang]['pass'] for task in tasks)}/{len(tasks)} passed")


if __name__ == "__main__":
    main()
