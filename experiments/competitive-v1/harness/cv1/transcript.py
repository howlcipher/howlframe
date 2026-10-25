"""Per-run accounting from the proxy's call archive (protocol §6 productivity and diagnostic measures).

    python3 -m cv1.transcript RUN_DIR [RUN_DIR ...]

Reads `records/proxy_calls.jsonl` and `records/calls/NNNN.req.json.gz` written by the proxy, and reports for each run:

* tokens by class and cost, summed from the provider-reported usage of every call;
* documentation reading: tool calls whose command or path reads the arm's documentation (/opt/docs, pydoc, go doc,
  `--help`), with their result sizes. Tokens for those results are an estimate (bytes / 4), reported as such;
* active time: wall time minus the union of intervals spent waiting on the provider;
* the first error the agent hit: the first tool result flagged as an error or reporting a non-zero exit code, for the
  blind diagnostic-quality grading.

Tool calls are collected from every archived request, keyed by call ID, so history the CLI later compacts is not lost.
"""

import argparse
import glob
import gzip
import json
import os
import re
import sys

DOC_PATTERNS = [re.compile(p) for p in (
    r"/opt/docs\b", r"\bpydoc\b", r"\bgo\s+doc\b", r"(^|[\s\"'])--help\b", r"\bhowlframe\s+help\b")]
EXIT_PATTERNS = [re.compile(p) for p in (
    r'"exit_code"\s*:\s*([1-9]\d*)', r"\bExit code:?\s*([1-9]\d*)", r"\bexited with (?:code|status)\s*([1-9]\d*)")]
BYTES_PER_TOKEN_ESTIMATE = 4
ERROR_TEXT_LIMIT = 4000


def _text(content):
    """Flatten a tool result's content (string, or list of text blocks) to text."""
    if content is None:
        return ""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "\n".join(_text(c.get("text", c.get("output", ""))) if isinstance(c, dict) else str(c)
                         for c in content)
    if isinstance(content, dict):
        return _text(content.get("text") or content.get("output") or content.get("content"))
    return str(content)


def _anthropic_items(body):
    for m in body.get("messages") or []:
        content = m.get("content")
        if not isinstance(content, list):
            continue
        for blk in content:
            if not isinstance(blk, dict):
                continue
            if blk.get("type") == "tool_use":
                yield "call", blk.get("id"), {"name": blk.get("name"), "input": json.dumps(blk.get("input"))}
            elif blk.get("type") == "tool_result":
                yield "result", blk.get("tool_use_id"), {"text": _text(blk.get("content")),
                                                         "is_error": bool(blk.get("is_error"))}


def _openai_items(body):
    for it in body.get("input") or []:
        if not isinstance(it, dict):
            continue
        t = it.get("type") or ""
        cid = it.get("call_id") or it.get("id")
        if t.endswith("_call_output"):
            yield "result", cid, {"text": _text(it.get("output")), "is_error": False}
        elif t.endswith("_call"):
            raw = it.get("arguments") if "arguments" in it else it.get("input", it.get("action"))
            yield "call", cid, {"name": it.get("name") or t, "input": raw if isinstance(raw, str) else json.dumps(raw)}


def load(run_dir):
    records = os.path.join(run_dir, "records")
    calls = []
    log = os.path.join(records, "proxy_calls.jsonl")
    if os.path.exists(log):
        with open(log) as f:
            calls = [json.loads(line) for line in f if line.strip()]
    bodies = []
    for path in sorted(glob.glob(os.path.join(records, "calls", "*.req.json.gz"))):
        with gzip.open(path) as f:
            bodies.append(json.loads(f.read()))
    return calls, bodies


def tool_events(provider, bodies):
    """Ordered tool calls, each with its result if one was sent back, from the union of all archived requests."""
    items = _anthropic_items if provider == "anthropic" else _openai_items
    order, calls, results = [], {}, {}
    for body in bodies:
        for kind, cid, data in items(body):
            if cid is None:
                continue
            if kind == "call" and cid not in calls:
                calls[cid] = data
                order.append(cid)
            elif kind == "result" and cid not in results:
                results[cid] = data
    return [dict(calls[c], id=c, result=results.get(c)) for c in order]


def is_doc_read(event):
    return any(p.search(event["input"] or "") for p in DOC_PATTERNS)


def is_error(event):
    r = event["result"]
    if not r:
        return False
    if "task-complete" in (event["input"] or ""):
        return False
    return r["is_error"] or any(p.search(r["text"]) for p in EXIT_PATTERNS)


def provider_wait_s(calls):
    """Length of the union of [start, start + duration] over forwarded calls."""
    spans = sorted((c["ts_start"], c["ts_start"] + (c.get("duration_s") or 0)) for c in calls if "ts_start" in c)
    total, cur_s, cur_e = 0.0, None, None
    for s, e in spans:
        if cur_e is None or s > cur_e:
            if cur_e is not None:
                total += cur_e - cur_s
            cur_s, cur_e = s, e
        else:
            cur_e = max(cur_e, e)
    if cur_e is not None:
        total += cur_e - cur_s
    return total


def _flat(usage, prefix=""):
    """Numeric usage fields, nested ones (OpenAI input_tokens_details.cached_tokens) as dotted names."""
    for k, v in usage.items():
        if isinstance(v, dict):
            yield from _flat(v, f"{prefix}{k}.")
        elif isinstance(v, (int, float)) and not isinstance(v, bool):
            yield prefix + k, v


def summarize(run_dir):
    with open(os.path.join(run_dir, "result.json")) as f:
        result = json.load(f)
    provider = result["agent"]["provider"]
    calls, bodies = load(run_dir)
    forwarded = [c for c in calls if "refused" not in c]
    tokens = {}
    for c in forwarded:
        for k, v in _flat(c.get("usage") or {}):
            tokens[k] = tokens.get(k, 0) + v
    events = tool_events(provider, bodies)
    docs = [e for e in events if is_doc_read(e)]
    doc_bytes = sum(len((e["result"] or {}).get("text", "").encode()) for e in docs)
    first_error = next((e for e in events if is_error(e)), None)
    wait = provider_wait_s(forwarded)
    return {
        "run_id": result["run_id"], "arm": result["arm"], "provider": provider,
        "model_calls": len(forwarded), "refused_calls": len(calls) - len(forwarded),
        "archived_requests": len(bodies), "tokens": tokens,
        "cost_usd": round(sum(c.get("cost_usd") or 0 for c in forwarded), 6),
        "tool_calls": len(events),
        "doc_reads": {"calls": len(docs), "result_bytes": doc_bytes,
                      "estimated_tokens": doc_bytes // BYTES_PER_TOKEN_ESTIMATE,
                      "estimate_basis": f"result bytes / {BYTES_PER_TOKEN_ESTIMATE}"},
        "wall_s": result["elapsed_s"], "provider_wait_s": round(wait, 3),
        "active_s": round(max(0.0, result["elapsed_s"] - wait), 3),
        "first_error": None if first_error is None else {
            "tool": first_error["name"], "command": first_error["input"][:ERROR_TEXT_LIMIT],
            "output": first_error["result"]["text"][:ERROR_TEXT_LIMIT]},
    }


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("runs", nargs="+")
    args = p.parse_args(argv)
    print(json.dumps([summarize(r) for r in args.runs], indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
