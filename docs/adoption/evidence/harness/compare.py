"""Black-box comparison: HowlBoard ledger_import (incumbent) vs EvidenceDesk projector.

Compares only observable outputs: exit code, stdout/stderr, and the two native
stores each tool writes. Usage: python3 -I compare.py HOWLFRAME_BIN INCUMBENT.hfbc PROJECTOR.hfbc
"""
import json
import os
import subprocess
import sys
import tempfile

BIN, INC, PROJ = sys.argv[1:4]
BOARD_FIXTURE = sys.argv[4] if len(sys.argv) > 4 else None


def run_incumbent(ledger, limit):
    d = tempfile.mkdtemp()
    p = subprocess.run([BIN, "-run-bc", "-allow-caps", "filesystem,database", "-max-instructions", "200000000",
                        INC, ledger, str(limit)], cwd=d, capture_output=True, text=True)
    return p, d


def run_projector(ledger, limit):
    d = tempfile.mkdtemp()
    p = subprocess.run([BIN, "run", "--allow-caps",
                        f"filesystem:read={ledger},filesystem:read={d},filesystem:write={d},database",
                        "--max-instructions", "10000000", PROJ, ledger, d, str(limit)],
                       capture_output=True, text=True)
    return p, d


def load(d):
    out = {}
    for name in ("howlboard_missions.json", "howlboard_timeline.json"):
        path = os.path.join(d, name)
        out[name] = json.load(open(path)) if os.path.exists(path) else None
    return out


def write(lines):
    f = tempfile.NamedTemporaryFile("w", suffix=".jsonl", delete=False)
    f.write("".join(lines))
    f.close()
    return f.name


def entry(i, task="T1", action="task_created", **kw):
    e = {"schema": "ai.evidence_entry/v1", "entry_id": f"e{i}", "task_id": task, "agent_id": "a", "action": action,
         "timestamp": "2026-10-01T00:00:00+00:00", "metadata": {}}
    e.update(kw)
    return json.dumps(e) + "\n"


def summarize(p, d):
    s = load(d)
    m = s["howlboard_missions.json"]
    t = s["howlboard_timeline.json"]
    return {
        "exit": p.returncode,
        "stdout": p.stdout.strip()[:120],
        "stderr": p.stderr.strip()[:160],
        "missions": None if m is None else len([k for k in m if k.startswith("mission:")]),
        "events": None if t is None else len(t["feed"]["events"]),
    }


cases = {}
cases["valid_3"] = (write([entry(1, "A"), entry(2, "A", "verification_completed", result="claimed",
                                                   verification_summary={"scope": "x"}),
                           entry(3, "B", "task_completed", result="accepted")]), 100)
cases["empty_file"] = (write([]), 100)
cases["blank_lines_only"] = (write(["\n", "   \n"]), 100)
cases["malformed_line_2"] = (write([entry(1), "{not json\n", entry(3)]), 100)
cases["wrong_schema"] = (write([entry(1).replace("ai.evidence_entry/v1", "other/v9")]), 100)
cases["schema_valid_no_metadata"] = (write([json.dumps({"schema": "ai.evidence_entry/v1", "entry_id": "x", "task_id": "A",
    "agent_id": "a", "action": "task_created", "timestamp": "t"}) + "\n"]), 100)
cases["missing_task_id"] = (write([json.dumps({"schema": "ai.evidence_entry/v1", "entry_id": "x", "agent_id": "a",
                                               "action": "task_created", "metadata": {}}) + "\n", entry(2)]), 100)
cases["duplicate_entry_id"] = (write([entry(1), entry(1, "B")]), 100)
cases["conflicting_state_order"] = (write([entry(1, "A", "task_completed"), entry(2, "A", "task_failed")]), 100)
cases["limit_exact_5_of_5"] = (write([entry(i, f"T{i}") for i in range(5)]), 5)
cases["limit_over_6_of_5"] = (write([entry(i, f"T{i}") for i in range(6)]), 5)
cases["human_approval_rejection"] = (write([entry(1, "A", "human_approval"), entry(2, "B", "human_rejection")]), 100)
big = [entry(i, f"T{i % 50}", "task_created", artifact="x" * 700) for i in range(400)]
cases["boundary_400x~830B_(>256KiB)"] = (write(big), 1000)
cases["1001_records"] = (write([entry(i, f"T{i % 20}") for i in range(1001)]), 2000)
if BOARD_FIXTURE:
    cases["board_sanitized_72"] = (BOARD_FIXTURE, 2000)

rows = []
for name, (ledger, limit) in cases.items():
    pi, di = run_incumbent(ledger, limit)
    pp, dp = run_projector(ledger, limit)
    rows.append((name, summarize(pi, di), summarize(pp, dp)))

for name, a, b in rows:
    print(f"## {name}")
    print(f"  incumbent: {json.dumps(a)}")
    print(f"  projector: {json.dumps(b)}")
