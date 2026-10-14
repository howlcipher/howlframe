#!/usr/bin/env python3
"""Developer-experience (gate H, items 1-4) black-box checker. Python 3 stdlib only.

Usage:  HF_BIN=/path/to/howlframe check_dx.py [-v] [--only NAME] [--list]

For every fixtures/NAME.howl with a sidecar NAME.expect.json this runs
`HF_BIN <command> fixtures/NAME.howl` (check, and build with -o <tmpfile>) and asserts the
combined stdout+stderr against the expectation file:

  commands                 required commands (each is a separate PASS/FAIL line)
  optional_commands        run and reported (INFO), never fail the checker
  expect_ok                true: exit 0 ; false: exit nonzero
  must_match               regular expressions (re.search, re.MULTILINE) that MUST match the output
  must_not_match           regular expressions that MUST NOT match the output
  must_differ_from_old_output   the output must not equal (after strip) the recorded "old_output"
                                (the diagnostic of the unmodified toolchain, see BASELINE.md)

Exit status: 0 all required checks pass, 1 any failure, 2 usage error.
It is expected to FAIL against the unmodified compiler (BASELINE.md).
"""
import argparse
import glob
import json
import os
import re
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
FIX = os.path.join(HERE, "fixtures")


def run(hf, cmd, path, tmpdir):
    args = [hf, cmd, path]
    if cmd == "build":
        args += ["-o", os.path.join(tmpdir, "out.hfbc")]
    try:
        p = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           cwd=os.path.dirname(path), timeout=60)
    except subprocess.TimeoutExpired:
        return None, "timeout"
    return p.returncode, (p.stdout + p.stderr).decode("utf-8", "replace")


def evaluate(exp, rc, out):
    """Return list of failure strings (empty means pass)."""
    bad = []
    if rc is None:
        return ["timed out"]
    if exp.get("expect_ok"):
        if rc != 0:
            bad.append("expected exit 0, got %d" % rc)
    elif rc == 0:
        bad.append("expected a nonzero exit, got 0")
    for pat in exp.get("must_match", []):
        if not re.search(pat, out, re.M):
            bad.append("output lacks /%s/" % pat)
    for pat in exp.get("must_not_match", []):
        if re.search(pat, out, re.M):
            bad.append("output must not match /%s/" % pat)
    old = exp.get("old_output")
    if exp.get("must_differ_from_old_output") and old is not None and out.strip() == old.strip():
        bad.append("output is identical to the old (unimproved) diagnostic")
    return bad


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("-v", action="store_true")
    ap.add_argument("--only", action="append", default=[])
    ap.add_argument("--list", action="store_true")
    a = ap.parse_args()
    names = sorted(os.path.basename(p)[:-len(".expect.json")]
                   for p in glob.glob(os.path.join(FIX, "*.expect.json")))
    if a.list:
        for n in names:
            e = json.load(open(os.path.join(FIX, n + ".expect.json")))
            print("%s\titem %s\t%s" % (n, e.get("item"), e.get("description", "")))
        return 0
    hf = os.environ.get("HF_BIN")
    if not hf or not os.access(hf, os.X_OK):
        print("error: set HF_BIN to the howlframe binary", file=sys.stderr)
        return 2
    failed = total = 0
    with tempfile.TemporaryDirectory(prefix="dxchk-") as tmp:
        for n in names:
            if a.only and n not in a.only:
                continue
            exp = json.load(open(os.path.join(FIX, n + ".expect.json")))
            src = os.path.join(FIX, n + ".howl")
            for cmd in exp.get("commands", []) + exp.get("optional_commands", []):
                optional = cmd in exp.get("optional_commands", [])
                rc, out = run(hf, cmd, src, tmp)
                bad = evaluate(exp, rc, out)
                if optional:
                    print("%s %s/%s (optional)" % ("INFO-ok " if not bad else "INFO-no ", n, cmd))
                    continue
                total += 1
                failed += bool(bad)
                print("%s %s/%s [item %s]" % ("PASS" if not bad else "FAIL", n, cmd, exp.get("item")))
                if bad and a.v:
                    for b in bad:
                        print("    - " + b)
                    print("    output: " + out.strip().replace("\n", "\n            ")[:900])
    print("\n%d checks, %d passed, %d failed" % (total, total - failed, failed))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
