"""Old-vs-new differential for HowlBoard's ledger import (observable behavior only).

Both importers run the same way (`-run-bc -allow-caps filesystem,database`, the
Makefile's instruction budget, cwd = a fresh directory holding the stores). For
every case it compares exit status, stdout, stderr and the complete decoded
contents of both stores, and for failing inputs checks that a prior import's
stores are byte-identical afterwards.

usage: python3 -I board_old_vs_new.py HOWLFRAME_BIN OLD.hfbc NEW.hfbc SANITIZED_FIXTURE
"""
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile

BIN, OLD, NEW, FIXTURE = sys.argv[1:5]
STORES = ("howlboard_missions.json", "howlboard_timeline.json")


def entry(i, task="T1", action="task_created", **kw):
    e = {"schema": "ai.evidence_entry/v1", "entry_id": f"e{i}", "task_id": task, "agent_id": "a",
         "action": action, "timestamp": "2026-10-01T00:00:00+00:00", "metadata": {}}
    e.update(kw)
    return json.dumps(e) + "\n"


def write(text):
    f = tempfile.NamedTemporaryFile("w", suffix=".jsonl", delete=False)
    f.write(text)
    f.close()
    return f.name


def run(artifact, ledger, limit, cwd):
    args = [BIN, "-run-bc", "-allow-caps", "filesystem,database", "-max-instructions", "200000000", artifact, ledger]
    if limit is not None:
        args.append(str(limit))
    p = subprocess.run(args, cwd=cwd, capture_output=True, text=True)
    stores = {}
    for name in STORES:
        path = os.path.join(cwd, name)
        stores[name] = json.load(open(path)) if os.path.exists(path) else None
    return p.returncode, p.stdout.strip(), p.stderr.strip(), stores


def digest(cwd):
    return {n: hashlib.sha256(open(os.path.join(cwd, n), "rb").read()).hexdigest()
            for n in STORES if os.path.exists(os.path.join(cwd, n))}


cases = {
    "sanitized_fixture_72": (FIXTURE, 2000),
    "sanitized_fixture_default_max": (FIXTURE, None),
    "valid_mixed": (write(entry(1, "A") + entry(2, "A", "verification_completed", result="failed",
                                               verification_summary={"exit_code": 7}) +
                          entry(3, "B", "task_completed", result="accepted")), 100),
    "conflicting_state_order": (write(entry(1, "A", "task_completed") + entry(2, "A", "task_failed")), 100),
    "approval_rejection_recovered": (write(entry(1, "A", "human_approval") + entry(2, "B", "human_rejection") +
                                           entry(3, "C", "implementation_recovered")), 100),
    "blank_lines_between": (write(entry(1, "A") + "\n  \n" + entry(2, "B")), 100),
    "exact_limit_5_of_5": (write("".join(entry(i, f"T{i}") for i in range(5))), 5),
    "over_limit_6_of_5": (write("".join(entry(i, f"T{i}") for i in range(6))), 5),
    "limit_zero": (write(entry(1)), 0),
    "limit_negative": (write(entry(1)), -5),
    "limit_not_integer": (write(entry(1)), "abc"),
    "empty_file": (write(""), 100),
    "blank_only": (write("\n   \n"), 100),
    "malformed_line_2": (write(entry(1) + "{not json\n" + entry(3)), 100),
    "truncated_last_line": (write(entry(1) + entry(2)[:40]), 100),
    "wrong_schema": (write(entry(1).replace("ai.evidence_entry/v1", "other/v9")), 100),
    "missing_task_id": (write(json.dumps({"schema": "ai.evidence_entry/v1", "entry_id": "x", "agent_id": "a",
                                          "action": "task_created", "metadata": {}}) + "\n"), 100),
    "non_object_line": (write("[1,2]\n"), 100),
    "duplicate_entry_id": (write(entry(1) + entry(1, "B")), 100),
    "schema_valid_no_metadata": (write(json.dumps({"schema": "ai.evidence_entry/v1", "entry_id": "x", "task_id": "A",
                                                   "agent_id": "a", "action": "task_created", "timestamp": "t"}) + "\n"), 100),
    "metadata_not_object": (write(entry(1, metadata="text")), 100),
    "missing_file": ("/nonexistent/ledger.jsonl", 100),
}

prior = tempfile.mkdtemp()
subprocess.run([BIN, "-run-bc", "-allow-caps", "filesystem,database", "-max-instructions", "200000000", OLD,
                write(entry(900, "PRIOR")), "10"], cwd=prior, check=True, capture_output=True)
prior_digest = digest(prior)

results = []
for name, (ledger, limit) in cases.items():
    row = {"case": name}
    for label, artifact in (("old", OLD), ("new", NEW)):
        cwd = tempfile.mkdtemp()
        rc, out, err, stores = run(artifact, ledger, limit, cwd)
        row[label] = {"exit": rc, "stdout": out, "stderr": err[:200],
                      "missions": None if stores[STORES[0]] is None else len(stores[STORES[0]]),
                      "events": None if stores[STORES[1]] is None else len(stores[STORES[1]]["feed"]["events"])}
        row[label + "_stores"] = stores
        # The same input against a directory holding a prior import.
        cwd2 = tempfile.mkdtemp()
        for n in STORES:
            shutil.copy(os.path.join(prior, n), cwd2)
        rc2, _, _, _ = run(artifact, ledger, limit, cwd2)
        row[label]["prior_store_unchanged"] = digest(cwd2) == prior_digest
    row["stores_equal"] = row["old_stores"] == row["new_stores"]
    del row["old_stores"], row["new_stores"]
    results.append(row)

# Repeated execution of the new importer into one directory is idempotent.
cwd = tempfile.mkdtemp()
run(NEW, FIXTURE, 2000, cwd)
first = digest(cwd)
run(NEW, FIXTURE, 2000, cwd)
idempotent = digest(cwd) == first

for r in results:
    print(json.dumps(r))
print(json.dumps({"new_repeated_import_idempotent": idempotent}))
