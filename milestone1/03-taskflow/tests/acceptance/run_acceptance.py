#!/usr/bin/env python3
"""Black-box acceptance suite for taskflow (milestone1/03-taskflow).

Usage:  run_acceptance.py LAUNCHER        run all tests
        run_acceptance.py --list          list test ids (no launcher needed)
        run_acceptance.py LAUNCHER --only T-R4 -v

Launcher contract: LAUNCHER GRANTS [APP_ARG...]   (REQUIREMENTS.md 1.2)
Child programs used by the Flowfiles are the Python interpreter running this suite
(canonical path, granted with process:exec=<path> **).  Python 3 standard library only.
"""
import json
import os
import re
import shutil
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from harness import (test, check, rc_is, no_runtime_leak, clean_denial, main, BASE_FLAGS)  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
FIX = os.path.join(HERE, "fixtures")
T = "taskflow"
PY = os.path.realpath(sys.executable)
SH = os.path.realpath(shutil.which("sh") or "/bin/sh")
PX = "process:exec=%s **" % PY


# ------------------------------------------------------------------ helpers
def put(d, name, text):
    p = os.path.join(d, name)
    with open(p, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    return p


def fx(name, **subs):
    with open(os.path.join(FIX, name), encoding="utf-8", newline="") as f:
        t = f.read()
    subs.setdefault("PY", PY)
    for k, v in subs.items():
        t = t.replace("@%s@" % k, str(v))
    return t


def rd(d):
    return "filesystem:read=" + d


def rw(d):
    return "filesystem:read=%s,filesystem:write=%s" % (d, d)


def join(*gs):
    return ",".join(g for g in gs if g)


def py(code):
    """A run line executing python code (code must avoid double quotes and backslashes)."""
    assert '"' not in code and "\\" not in code
    return '  run %s -c "%s"' % (PY, code)


def flow(*lines):
    return "\n".join(lines) + "\n"


def shq(w):
    return w if re.fullmatch(r"[A-Za-z0-9_@%+=:,./-]+", w) else "'" + w.replace("'", "'\\''") + "'"


def blk(name, out=(), status="ok"):
    s = "==> %s\n" % name + "".join("| %s\n" % ln for ln in out)
    return s + "<== %s %s\n" % (name, status)


def skipped(name, why):
    return "<== %s skipped (%s)\n" % (name, why)


def summary(ok=0, failed=0, sk=0):
    return "SUMMARY ok=%d failed=%d skipped=%d\n" % (ok, failed, sk)


def fail_clean(res, rc, prefix=T + ": "):
    rc_is(res, rc)
    check(res.out == "", "stdout must be empty", res)
    check(res.err != "", "diagnostic expected", res)
    for ln in res.err.splitlines():
        check(ln.startswith(prefix), "stderr line without '%s': %r" % (prefix, ln), res)
    no_runtime_leak(res)


def runff(c, text, args=("run",), grants=PX, extra=(), **kw):
    """Run taskflow with the Flowfile on stdin."""
    a = [args[0], "-f", "-"] + list(args[1:]) + list(extra)
    return c.run(grants, a, stdin=text, **kw)


def hexargs(*xs):
    return json.dumps([x.encode("utf-8").hex() for x in xs])


# ------------------------------------------------------------------ R1 CLI
@test("T-R1-version-help", ["R1"], "version string; help usage text; needs no grants")
def _(c):
    r = c.run("", ["version"])
    rc_is(r, 0)
    check(r.out == "taskflow 1.0.0\n", "version", r)
    for a in ("help", "--help", "-h"):
        r = c.run("", [a])
        rc_is(r, 0)
        check(r.out.startswith("usage: taskflow"), "usage", r)


@test("T-R1-usage-errors", ["R1", "R15"], "usage errors: exit 2, no stdout, no I/O, nothing executed")
def _(c):
    d = c.tmp()
    marker = os.path.join(d, "marker")
    ff = put(d, "Flowfile", flow("task t", py("open('%s','w').write('x')" % marker)))
    g = join(rw(d), PX)
    bad = [
        [], ["bogus"], ["version", "x"], ["history"], ["history", "a", "b"], ["history", "--last", "0", ff], ["history", "--last", "x", ff],
        ["run", "-f"], ["run", "-fFlowfile"], ["run", "--file=" + ff], ["run", "--bogus"], ["run", "-f", ff, "-f", ff],
        ["run", "-D", "noequals"], ["run", "-D", "1x=2"], ["run", "-D"], ["run", "-f", ff, "--report"],
        ["run", "-f", ff, "--keep-going", "--keep-going"], ["list", "-D", "A=1"], ["list", "--dry-run"], ["plan", "--report", "r"],
        ["list", "-f", ff, "extra"], ["run", "--last", "3"], ["history", "-f", ff, ff],
        ["run", "-f", ff],                         # no task given and no 'default'
        ["run", "-f", ff, "nosuchtask"],           # unknown task (after load)
    ]
    for a in bad:
        r = c.run(g, a, cwd=d)
        rc_is(r, 2)
        check(r.out == "", "stdout on usage error: %r" % a, r)
        check(r.err.startswith(T + ": "), "diagnostic prefix %r" % a, r)
        check(not os.path.exists(marker), "nothing may be executed on usage error: %r" % a, r)
    check(sorted(os.listdir(d)) == ["Flowfile"], "usage errors must not create files: %r" % os.listdir(d))


@test("T-R1-default-task-and-stdin", ["R1", "R5"], "default task used when none given; -f - reads stdin; -f file works with read grant")
def _(c):
    d = c.tmp()
    text = flow("task default after a", "task a", py("print(1)"))
    r = runff(c, text, ("run",))
    rc_is(r, 0)
    check(r.out == blk("a", ["1"]) + blk("default") + summary(ok=2), "output", r)
    p = put(d, "Flowfile", text)
    r = c.run(join(rd(d), PX), ["run", "-f", p])
    rc_is(r, 0)
    r2 = c.run(join(rd(d), PX), ["run"], cwd=d)       # default file name 'Flowfile' in cwd
    rc_is(r2, 0)
    check(r2.out == r.out, "default Flowfile name", r2)


# ------------------------------------------------------------------ R2 parsing
PARSE_ERRORS = [
    ("bogus line\n", 1), ("  run /x\n", 1), ("task a\n  frob x\n", 2), ("set 1X = a\n", 1), ("set X a\n", 1),
    ("set X = a\nset X = b\n", 2), ("task a\ntask a\n", 2), ("task a after\n", 1), ("task a after nope\n", 1),
    ("task a b\n", 1), ("task a after b!\n", 1), ("task\n", 1), ("task a$\n", 1), ("task a/b\n", 1),
    ("task a\n  desc one\n  desc two\n", 3), ("task a\n  run\n", 2), ("task a\n  run \"abc\n", 2),
    ("task a\n  run /x 'abc\n", 2), ("task a\n  run /x $y\n", 2), ("task a\n  run /x ${}\n", 2),
    ("task a\n  run /x ${env:}\n", 2), ("task a\n  run /x ${a b}\n", 2), ("task a\n  run /x abc$\n", 2),
    ("task a\n  run /x \"${\"\n", 2), ("set X = $y\n", 1), ("set X = ${\n", 1), ("set X = a\n\nbogus\n", 3),
    ("task a after nope\nbogus\n", 1), ("bogus\nset X = $y\n", 1), ("task a\ntask b after nope\n", 2),
    ("task a\n\n# c\nunknown_directive x\n", 4), ("task a\n  # fine\n  run\t\n", 3),
]
CYCLES = [
    ("task a after b\ntask b after a\n", 1), ("task a after b\ntask b after c\ntask c after a\n", 1), ("task a after a\n", 1),
    ("task ok\ntask x after y\ntask y after x\n", 2), ("task top after x\ntask x after y\ntask y after x\n", 2),
]


@test("T-R2-parse-errors", ["R2", "R5", "R14"], "every grammar violation: exit 65 with '-:LINE:' prefix, nothing on stdout")
def _(c):
    for text, line in PARSE_ERRORS:
        for cmd in ("plan", "list"):
            r = runff(c, text, (cmd,), grants="")
            fail_clean(r, 65)
            check(r.err.splitlines()[0].startswith("%s: -:%d: " % (T, line)), "%s: expected '-:%d:' for %r, got %r" % (cmd, line, text, r.err), r)
            check(len(r.err.splitlines()) == 1, "exactly one error line", r)


@test("T-R2-cycles", ["R2", "R5"], "dependency cycles are load errors naming the cycle, even if unreferenced")
def _(c):
    for text, line in CYCLES:
        r = runff(c, text, ("plan",), grants="", extra=())
        fail_clean(r, 65)
        check(r.err.startswith("%s: -:%d: " % (T, line)) and "cycle" in r.err, "cycle diagnostic for %r: %r" % (text, r.err), r)
    r = runff(c, "task a after b\ntask b after c\ntask c after a\n", ("plan",), grants="")
    check(all(n in r.err for n in "abc") and "->" in r.err, "cycle members listed", r)


@test("T-R2-filename-in-diagnostic", ["R2", "R13"], "diagnostic names the -f argument exactly; missing file => 66")
def _(c):
    d = c.tmp()
    p = put(d, "My Flow.txt", "task a\nbogus\n")
    r = c.run(rd(d), ["plan", "-f", p])
    fail_clean(r, 65)
    check(r.err.startswith("%s: %s:2: " % (T, p)), "diagnostic: %r" % r.err, r)
    fail_clean(c.run(rd(d), ["plan", "-f", os.path.join(d, "nope")]), 66)
    fail_clean(c.run(rd(d), ["plan"], cwd=d), 66)       # default 'Flowfile' missing


@test("T-R2-valid-formats", ["R2", "R5"], "CRLF, tabs, comments, blank lines, zero-run tasks, desc; list golden")
def _(c):
    text = ("# header comment\r\n\r\nset  A  =  a = b \r\ntask one\r\n\t# comment in body\r\n\tdesc  first task \r\n\trun /bin/true x # not a comment\r\n"
            "\r\ntask two after one\r\n  run /bin/true\r\ntask three   after   two   one\r\n")
    r = runff(c, text, ("list",), grants="")
    rc_is(r, 0)
    check(r.out == "one\t-\tfirst task\nthree\ttwo,one\t-\ntwo\tone\t-\n", "list output", r)
    r = runff(c, text, ("plan", "three"), grants="")
    rc_is(r, 0)
    check(r.out == "1\tone\n2\ttwo\n3\tthree\n", "plan output", r)
    d = c.tmp()
    p = put(d, "Flowfile", fx("Flowfile.basic"))
    r = c.run(rd(d), ["list", "-f", p])
    rc_is(r, 0)
    check(r.out == fx("Flowfile.basic.list.txt"), "list golden", r)
    check(r.err == "", "stderr empty", r)
    r = runff(c, "", ("list",), grants="")      # an empty Flowfile is valid
    rc_is(r, 0)
    check(r.out == "", "empty list", r)


# ------------------------------------------------------------------ R5 graph
@test("T-R5-plan-order", ["R5"], "post-order, declaration-order deps, no repeats, multiple/duplicate targets")
def _(c):
    ffile = flow("task d after b c", "task b after a", "task c after a", "task a", "task x", "task y after x d")
    cases = [
        (["d"], ["a", "b", "c", "d"]), (["y"], ["x", "a", "b", "c", "d", "y"]), (["c", "b"], ["a", "c", "b"]),
        (["b", "b", "b"], ["a", "b"]), (["x", "a", "x"], ["x", "a"]), (["a"], ["a"]),
    ]
    for targets, want in cases:
        r = runff(c, ffile, ("plan",), grants="", extra=targets)
        rc_is(r, 0)
        check(r.out == "".join("%d\t%s\n" % (i + 1, n) for i, n in enumerate(want)), "plan %r: %r" % (targets, r.out), r)
        check(r.err == "", "stderr", r)
    r = runff(c, ffile, ("plan",), grants="")
    fail_clean(r, 2)           # no default
    r = runff(c, ffile + "task default after y\n", ("plan",), grants="")
    check(r.out.splitlines()[-1] == "7\tdefault", "default target", r)


@test("T-R5-scale-chain", ["R5", "R15"], "300 chained tasks planned correctly; 1000 tasks fine, 1001 => exit 70")
def _(c):
    n = 300
    ffile = "task t0\n" + "".join("task t%d after t%d\n" % (i, i - 1) for i in range(1, n))
    r = runff(c, ffile, ("plan",), grants="", extra=["t%d" % (n - 1)], timeout=170)
    rc_is(r, 0)
    check(r.out == "".join("%d\tt%d\n" % (i + 1, i) for i in range(n)), "chain plan", r)
    r = runff(c, "".join("task t%d\n" % i for i in range(1000)), ("list",), grants="", timeout=170)
    rc_is(r, 0)
    check(len(r.out.splitlines()) == 1000, "1000 tasks", r)
    r = runff(c, "".join("task t%d\n" % i for i in range(1001)), ("list",), grants="", timeout=170)
    fail_clean(r, 70)
    check("task limit exceeded (max 1000)" in r.err, "message", r)


# ------------------------------------------------------------------ R3 variables / R8 dry-run
def dry(c, text, extra=(), grants="", env=None):
    return runff(c, text, ("run", "--dry-run"), grants=grants, extra=list(extra), env=env)


def would(*words):
    return "| would run: " + " ".join(shq(w) for w in words) + "\n"


@test("T-R3-variables-dry", ["R3", "R8"], "set, -D override, -D-only, recursion, $$, quoting contexts (dry-run shows expanded argv)")
def _(c):
    text = flow("set A = alpha", "set B = ${A}-beta", "set C = x  y", "set EMPTY =", "task t",
                "  run /bin/echo ${A} ${B} ${C} \"${C}\" '${C}' $$A \"$$\" a${EMPTY}b ${EMPTY} \"\" '' 'it''s' \"q\\\"uo\\\\te\" ${D}")
    r = dry(c, text, ["t", "-D", "D=from cli", "-D", "A=OVER"])
    rc_is(r, 0)
    # words: ${A}->OVER ; ${B}->OVER-beta ; ${C} (unquoted value stays ONE word) ; "${C}" -> one word ; '${C}' literal
    want_words = ["/bin/echo", "OVER", "OVER-beta", "x  y", "x  y", "${C}", "$A", "$", "ab", "", "", "", "its", 'q"uo\\te', "from cli"]
    # an unquoted word that expands to the empty string stays an (empty) argument
    check(r.out == "==> t\n" + would(*want_words) + "<== t dry-run\nSUMMARY dry-run tasks=1\n", "dry-run output:\n%s" % r.out, r)


@test("T-R3-variable-errors", ["R3"], "undefined variable / cycles found by run (exit 65, FILE:LINE), not by plan/list")
def _(c):
    cases = [
        ("task t\n  run /bin/echo ${NOPE}\n", 2, "undefined variable NOPE"),
        ("set A = ${B}\nset B = ${A}\ntask t\n  run /bin/echo ${A}\n", 1, "variable cycle"),
        ("set A = ${A}\ntask t\n  run /bin/echo ${A}\n", 1, "variable cycle"),
        ("set A = ${MISSING}\ntask t\n  run /bin/echo ${A}\n", 1, "undefined variable MISSING"),
    ]
    for text, line, msg in cases:
        for args in (("run",), ("run", "--dry-run")):
            r = runff(c, text, args, grants="", extra=["t"])
            fail_clean(r, 65)
            check(r.err.startswith("%s: -:%d: " % (T, line)) and msg in r.err, "expected -:%d: %s, got %r" % (line, msg, r.err), r)
        r = runff(c, text, ("plan",), grants="", extra=["t"])
        rc_is(r, 0)           # plan/list never expand
        r = runff(c, text, ("list",), grants="")
        rc_is(r, 0)
    # a -D definition removes the 'undefined' error / breaks the cycle
    r = runff(c, "task t\n  run /bin/echo ${NOPE}\n", ("run", "--dry-run"), grants="", extra=["t", "-D", "NOPE=1"])
    rc_is(r, 0)
    r = runff(c, "set A = ${B}\nset B = ${A}\ntask t\n  run /bin/echo ${A}\n", ("run", "--dry-run"), grants="", extra=["t", "-D", "A=ok"])
    rc_is(r, 0)
    check(would("/bin/echo", "ok") in r.out, "override breaks cycle", r)


@test("T-R8-dry-run-semantics", ["R8", "R12", "R11"], "dry-run: nothing executed, no process/env grants needed, ${env:} stays verbatim, relative program rejected")
def _(c):
    d = c.tmp()
    marker = os.path.join(d, "marker")
    text = flow("task t after u", py("open('%s','w').write('x')" % marker), "  run %s ${env:HOME} \"${env:A_B}\" '${env:Q}'" % PY,
                "task u", "  run %s -c 'print(1)'" % PY)
    r = dry(c, text, ["t"], grants="", env={"HOME": "/home/zzz-secret"})
    rc_is(r, 0)
    check(not os.path.exists(marker), "dry-run must not execute", r)
    check("zzz-secret" not in r.out + r.err, "environment must not be read in dry-run", r)
    lines = r.out.splitlines()
    check(lines[0] == "==> u" and lines[-1] == "SUMMARY dry-run tasks=2" and lines[-2] == "<== t dry-run", "structure %r" % lines, r)
    check(would(PY, "${env:HOME}", "${env:A_B}", "${env:Q}") in r.out, "env refs verbatim: %r" % r.out, r)
    check(r.err == "", "stderr empty", r)
    # relative program is rejected in dry-run too
    r = dry(c, "task t\n  run echo hi\n", ["t"])
    fail_clean(r, 65)
    check("program must be an absolute path" in r.err and r.err.startswith("%s: -:2: " % T), "relative program: %r" % r.err, r)
    # dry-run never writes report/history
    rp = os.path.join(d, "report.json")
    hp = os.path.join(d, "hist.json")
    r = runff(c, text, ("run", "--dry-run", "--report", rp, "--history", hp), grants="", extra=["t"])
    rc_is(r, 0)
    check(not os.path.exists(rp) and not os.path.exists(hp), "dry-run must not write report/history", r)


# ------------------------------------------------------------------ R4 argv fidelity (real exec)
@test("T-R4-argv-fidelity", ["R4", "R3", "R11"], "child argv equals the expanded words exactly (quotes, blanks, empty args, UTF-8, no re-splitting)")
def _(c):
    d = c.tmp()
    dump = put(d, "argv_dump.py", fx("argv_dump.py"))
    text = flow('set SP = a  b "c" \'d\' $$x', "set U = café €", "task t",
                "  run %s %s plain \"dq with  spaces\" 'sq with $dollar' a\"b c\"'d e'f ${SP} \"${SP}\" ${U} \"\" '' ${CLI} \"x\\\"y\\\\z\" tab\tsep $$" % (PY, dump))
    r = runff(c, text, ("run", "t", "-D", "CLI=line1\nline2"), grants=PX)
    rc_is(r, 0)
    # NB: the two empty-word arguments ("" and '') are separate empty arguments
    want = hexargs("plain", "dq with  spaces", "sq with $dollar", "ab cd ef", 'a  b "c" \'d\' $x', 'a  b "c" \'d\' $x',
                   "café €", "", "", "line1\nline2", 'x"y\\z', "tab", "sep", "$")
    check(r.out == blk("t", [want]) + summary(ok=1), "argv mismatch.\nwant %s\ngot  %s" % (want, r.out), r)


@test("T-R4-no-shell", ["R4", "R15"], "shell metacharacters in arguments have no effect")
def _(c):
    d = c.tmp()
    dump = put(d, "argv_dump.py", fx("argv_dump.py"))
    pwn = os.path.join(d, "pwned")
    nasty = ["a;touch " + pwn, "x|y", "$(touch %s)" % pwn, "`touch %s`" % pwn, "*", "?.txt", "> " + pwn, "<in", "&& touch " + pwn, "~", "\\n", "{a,b}"]
    words = " ".join("'%s'" % n for n in nasty)
    open(os.path.join(d, "present.txt"), "w").close()
    text = flow("task t", "  run %s %s %s" % (PY, dump, words), "  run %s %s ; touch %s | cat > %s *" % (PY, dump, pwn, pwn))
    r = c.run(join(PX, rd(d)), ["run", "-f", "-", "t"], stdin=text, cwd=d)
    rc_is(r, 0)
    outs = [ln for ln in r.out.splitlines() if ln.startswith("| ")]
    check(outs[0] == "| " + hexargs(*nasty), "first command argv", r)
    check(outs[1] == "| " + hexargs(";", "touch", pwn, "|", "cat", ">", pwn, "*"), "second command argv: %r" % outs[1], r)
    check(not os.path.exists(pwn), "injection created a file", r)


@test("T-R4-program-checks", ["R4", "R6", "R7"], "relative program => 65 before anything runs; missing absolute program => FAILED (error)")
def _(c):
    d = c.tmp()
    marker = os.path.join(d, "marker")
    text = flow("task a", py("open('%s','w').write('x')" % marker), "task b after a", "  run python3 -c 'print(1)'")
    r = runff(c, text, ("run", "b"))
    fail_clean(r, 65)
    check(r.err.startswith("%s: -:4: " % T) and "program must be an absolute path" in r.err, "diagnostic %r" % r.err, r)
    check(not os.path.exists(marker), "nothing may run when planning fails", r)
    missing = os.path.join(d, "no-such-program")
    r = runff(c, flow("task m", "  run %s" % missing), ("run", "m"), grants="process:exec=%s **" % missing)
    check(r.rc in (1, 77) and r.out.startswith("==> m\n<== m FAILED (") and "SUMMARY ok=0 failed=1 skipped=0" in r.out, "missing program output %r" % r.out, r)
    no_runtime_leak(r)
    r = runff(c, flow("task m", "  run %s ${X}" % PY), ("run", "m", "-D", "X=--version"), grants="process:exec=%s -c print(1)" % PY)
    rc_is(r, 77)         # argv not in allow-list => denied, not 'error'
    check("FAILED (denied)" in r.out, "denied status", r)


# ------------------------------------------------------------------ R6 / R7 execution
@test("T-R6-basic-run-golden", ["R6", "R5"], "golden output of a successful pipeline, exit 0")
def _(c):
    d = c.tmp()
    p = put(d, "Flowfile", fx("Flowfile.basic"))
    r = c.run(join(rd(d), PX), ["run", "-f", p, "default"])
    rc_is(r, 0)
    want = (blk("gen", ["gen:hello world"]) + blk("build", ["build ok"]) + blk("test", ["t1", "t2"]) + blk("default") + summary(ok=4))
    check(r.out == want, "stdout differs", r)
    check(r.err == "", "stderr must be empty", r)
    r = c.run(join(rd(d), PX), ["run", "-f", p, "package"])
    rc_is(r, 0)
    check(r.out == blk("gen", ["gen:hello world"]) + blk("build", ["build ok"]) + blk("test", ["t1", "t2"]) + blk("package") + summary(ok=4), "package", r)


@test("T-R6-output-shapes", ["R6"], "output without trailing newline, empty lines, interleaved stderr, no output, exit codes")
def _(c):
    text = flow(
        "task nonl", py("import sys; sys.stdout.write('no newline')"),
        "task empties", py("print(); print('x'); print()"),
        "task merged", py("import sys; print('o1', flush=True); print('e1', file=sys.stderr, flush=True); print('o2', flush=True)"),
        "task quiet", py("pass"),
        "task two", py("print('a')"), py("print('b')"),
        "task code3", py("import sys; print('boom'); sys.exit(3)"),
        "task code255", py("import sys; sys.exit(255)"))
    r = runff(c, text, ("run", "nonl", "empties", "merged", "quiet", "two"))
    rc_is(r, 0)
    want = (blk("nonl", ["no newline"]) + blk("empties", ["", "x", ""]) + blk("merged", ["o1", "e1", "o2"]) + blk("quiet") + blk("two", ["a", "b"]) + summary(ok=5))
    check(r.out == want, "shapes", r)
    r = runff(c, text, ("run", "code3"))
    rc_is(r, 1)
    check(r.out == blk("code3", ["boom"], "FAILED (exit 3)") + summary(failed=1), "failure output must show the child's output and exit code", r)
    r = runff(c, text, ("run", "code255"))
    rc_is(r, 1)
    check(r.out == blk("code255", [], "FAILED (exit 255)") + summary(failed=1), "exit 255", r)


@test("T-R7-fail-fast", ["R7", "R6"], "fail-fast: first failure aborts, later tasks skipped (aborted), later commands of a failed task not run")
def _(c):
    d = c.tmp()
    m = os.path.join(d, "m")
    text = flow("task a", py("print('a')"), "task bad", py("import sys; print('x'); sys.exit(2)"), py("open('%s','w').write('1')" % m),
                "task c after bad", py("print('c')"), "task e", py("print('e')"), "task all after a bad c e")
    r = runff(c, text, ("run", "all"))
    rc_is(r, 1)
    want = blk("a", ["a"]) + blk("bad", ["x"], "FAILED (exit 2)") + skipped("c", "aborted") + skipped("e", "aborted") + skipped("all", "aborted") + summary(ok=1, failed=1, sk=3)
    check(r.out == want, "fail-fast output", r)
    check(not os.path.exists(m), "later command of a failed task executed", r)


@test("T-R7-keep-going", ["R7", "R6"], "--keep-going: independent branches run; dependents skipped naming the first non-ok dependency")
def _(c):
    text = flow("task a", py("print('a')"), "task bad", py("import sys; sys.exit(1)"), "task c after bad", py("print('c')"),
                "task e", py("print('e')"), "task d after a c e", py("print('d')"), "task f after e a", py("print('f')"),
                "task g after d", py("print('g')"), "task all after a bad c e d f g")
    r = runff(c, text, ("run", "--keep-going", "all"))
    rc_is(r, 1)
    want = (blk("a", ["a"]) + blk("bad", [], "FAILED (exit 1)") + skipped("c", "dependency bad failed") + blk("e", ["e"])
            + skipped("d", "dependency c failed") + blk("f", ["f"]) + skipped("g", "dependency d failed")
            + skipped("all", "dependency bad failed") + summary(ok=3, failed=1, sk=4))
    check(r.out == want, "keep-going output", r)


@test("T-R6-exit-and-unknown-failure-kinds", ["R6", "R7"], "exit status 0 only when every task ok; summary counts")
def _(c):
    r = runff(c, flow("task a", py("print(1)"), "task b", py("import sys; sys.exit(9)")), ("run", "a", "b", "--keep-going"))
    rc_is(r, 1)
    check(r.out == blk("a", ["1"]) + blk("b", [], "FAILED (exit 9)") + summary(ok=1, failed=1), "output", r)
    r = runff(c, flow("task a", py("print(1)")), ("run", "a", "a"))
    rc_is(r, 0)
    check(r.out == blk("a", ["1"]) + summary(ok=1), "duplicate target runs once", r)


# ------------------------------------------------------------------ R9 report / R10 history
@test("T-R9-report", ["R9", "R7"], "report JSON for ok and failed runs, written even on failure; plan order")
def _(c):
    d = c.tmp()
    rp = os.path.join(d, "report.json")
    text = flow("task a", py("print('a')"), "task bad after a", py("import sys; sys.exit(4)"), py("print('x')"),
                "task c after bad", py("print('c')"), "task e", "task d after e a", py("print('d')"))
    r = runff(c, text, ("run", "--keep-going", "--report", rp, "c", "d"), grants=join(PX, rw(d)))
    rc_is(r, 1)
    j = json.load(open(rp))
    check(j == {"version": 1, "status": "failed", "targets": ["c", "d"], "tasks": [
        {"name": "a", "status": "ok", "exit": 0, "reason": None, "commands": 1},
        {"name": "bad", "status": "failed", "exit": 4, "reason": "exit", "commands": 1},
        {"name": "c", "status": "skipped", "exit": None, "reason": "dependency_failed", "commands": 0},
        {"name": "e", "status": "ok", "exit": 0, "reason": None, "commands": 0},
        {"name": "d", "status": "ok", "exit": 0, "reason": None, "commands": 1}]}, "report: %r" % j, r)
    rp2 = os.path.join(d, "report2.json")
    r = runff(c, text, ("run", "--report", rp2, "a", "bad", "c"), grants=join(PX, rw(d)))
    rc_is(r, 1)
    j = json.load(open(rp2))
    check([(t["name"], t["status"], t["reason"]) for t in j["tasks"]] == [("a", "ok", None), ("bad", "failed", "exit"), ("c", "skipped", "aborted")], "aborted reasons: %r" % j, r)
    rp3 = os.path.join(d, "report3.json")
    r = runff(c, flow("task default", "task x", py("print(1)")), ("run", "--report", rp3), grants=join(PX, "filesystem:write=" + d))
    rc_is(r, 0)
    j = json.load(open(rp3))
    check(j["status"] == "ok" and j["targets"] == ["default"] and j["tasks"] == [{"name": "default", "status": "ok", "exit": 0, "reason": None, "commands": 0}], "ok report %r" % j, r)


@test("T-R10-history", ["R10"], "history append, formats, --last, trimming to 50, corrupt/missing handling")
def _(c):
    d = c.tmp()
    hp = os.path.join(d, "hist.json")
    g = join(PX, rw(d))
    t0 = int(time.time())
    ok = flow("task ok", py("print(1)"), "task bad", py("import sys; sys.exit(1)"))
    rc_is(runff(c, ok, ("run", "--history", hp, "ok"), grants=g), 0)
    rc_is(runff(c, ok, ("run", "--history", hp, "ok", "bad", "--keep-going"), grants=g), 1)
    rc_is(runff(c, ok, ("run", "--history", hp, "--dry-run", "ok"), grants=g), 0)   # dry-run: no record
    j = json.load(open(hp))
    check(j["version"] == 1 and len(j["runs"]) == 2, "two records expected: %r" % j, None)
    check([(x["targets"], x["status"], x["ok"], x["failed"], x["skipped"]) for x in j["runs"]] == [(["ok"], "ok", 1, 0, 0), (["ok", "bad"], "failed", 1, 1, 0)], "records", None)
    check(all(isinstance(x["started"], int) and t0 - 5 <= x["started"] <= t0 + 60 for x in j["runs"]), "started timestamps", None)
    r = c.run(rd(d), ["history", hp])
    rc_is(r, 0)
    rows = [ln.split("\t") for ln in r.out.splitlines()]
    check([row[1:] for row in rows] == [["ok", "ok", "1", "0", "0"], ["failed", "ok,bad", "1", "1", "0"]], "history rows %r" % rows, r)
    r = c.run(rd(d), ["history", "--last", "1", hp])
    check(len(r.out.splitlines()) == 1 and r.out.split("\t")[1] == "failed", "--last 1", r)
    fail_clean(c.run(rd(d), ["history", os.path.join(d, "none.json")]), 66)
    bad = put(d, "bad.json", '{"version": 1, "runs": [{"started": "x"}]}')
    fail_clean(c.run(rd(d), ["history", bad]), 65)
    marker = os.path.join(d, "ran")
    r = runff(c, flow("task t", py("open('%s','w').write('1')" % marker)), ("run", "--history", bad, "t"), grants=g)
    fail_clean(r, 65)
    check(not os.path.exists(marker), "nothing may run when the history file is invalid", r)
    # trimming to the last 50
    hp2 = os.path.join(d, "hist2.json")
    quiet = flow("task q")
    for _i in range(53):
        rc_is(runff(c, quiet, ("run", "--history", hp2, "q"), grants=rw(d)), 0)
    r = c.run(rd(d), ["history", hp2])
    check(len(r.out.splitlines()) == 50, "history must keep the last 50 runs, got %d" % len(r.out.splitlines()), r)
    ts = [int(x.split("\t")[0]) for x in r.out.splitlines()]
    check(ts == sorted(ts), "oldest first", r)


# ------------------------------------------------------------------ R11 process capability
@test("T-R11-process-denied", ["R11", "R6", "R7"], "no exec grant: FAILED (denied), one stderr line per denied task, exit 77, nothing executed")
def _(c):
    d = c.tmp()
    m = os.path.join(d, "m")
    text = flow("task a", py("open('%s','w').write('1')" % m), "task b after a", py("print(2)"))
    for grants in ("", "process", "process:exec=%s -c print(1)" % PY, "process:exec=%s **" % SH):
        r = runff(c, text, ("run", "--keep-going", "b"), grants=grants)
        rc_is(r, 77)
        check(r.out == "==> a\n<== a FAILED (denied)\n" + skipped("b", "dependency a failed") + summary(failed=1, sk=1), "stdout (grants=%r): %r" % (grants, r.out), r)
        check(r.err == "taskflow: capability denied: process (a)\n", "stderr: %r" % r.err, r)
        check(PY not in r.err and PY not in r.out and "open(" not in r.err, "program/args must not be echoed", r)
        check(not os.path.exists(m), "denied command must not run", r)
        no_runtime_leak(r)


@test("T-R11-exec-allow-list", ["R11", "R7"], "per-program / per-argv grants honored; denial does not stop independent tasks with --keep-going; 77 outranks 1")
def _(c):
    text = flow("task good", "  run %s -c print(1)" % PY, "task other", "  run %s -c print(2)" % PY, "task sh", "  run %s -c true" % SH,
                "task fail", py("import sys; sys.exit(1)"))
    r = runff(c, text, ("run", "--keep-going", "good", "other", "sh", "fail"), grants="process:exec=%s -c print(1)" % PY)
    rc_is(r, 77)
    st = {m.group(1): m.group(2) for m in re.finditer(r"^<== (\S+) (ok|FAILED \(\w+( \d+)?\)|skipped.*)$", r.out, re.M)}
    check(st.get("good") == "ok" and st.get("other") == "FAILED (denied)" and st.get("sh") == "FAILED (denied)" and st.get("fail") == "FAILED (denied)", "statuses %r" % st, r)
    check(r.err.splitlines() == ["taskflow: capability denied: process (other)", "taskflow: capability denied: process (sh)",
                                 "taskflow: capability denied: process (fail)"], "stderr %r" % r.err, r)
    # with the python grant open, only the genuine failure remains => exit 1
    r = runff(c, text, ("run", "--keep-going", "good", "other", "fail"), grants=PX)
    rc_is(r, 1)


@test("T-R11-env-isolation", ["R11", "R4"], "children get only process:env= variables; nothing else is inherited")
def _(c):
    d = c.tmp()
    dump = put(d, "env_dump.py", fx("env_dump.py"))
    text = flow("task t", "  run %s %s" % (PY, dump))
    env = {"HF_TEST_PASS": "pass-value", "HF_TEST_SECRET": "s3cret-value", "HOME": "/home/x", "PATH": os.environ.get("PATH", "/usr/bin")}
    r = runff(c, text, ("run", "t"), grants=join(PX, "process:env=HF_TEST_PASS"), env=env)
    rc_is(r, 0)
    names = json.loads(r.out.splitlines()[1][2:])
    check("HF_TEST_PASS" in names, "granted variable must reach the child: %r" % names, r)
    for bad in ("HF_TEST_SECRET", "HOME", "PATH", "HF_RUN_FLAGS"):
        check(bad not in names, "%s leaked into the child environment" % bad, r)
    r = runff(c, text, ("run", "t"), grants=PX, env=env)
    names = json.loads(r.out.splitlines()[1][2:])
    check("HF_TEST_PASS" not in names and "HF_TEST_SECRET" not in names, "no grants => no variables: %r" % names, r)


# ------------------------------------------------------------------ R12 environment
@test("T-R12-env-expansion-and-denial", ["R12", "R3", "R15"], "${env:} needs 'environment'; denial before anything runs; no secret leakage")
def _(c):
    d = c.tmp()
    dump = put(d, "argv_dump.py", fx("argv_dump.py"))
    m = os.path.join(d, "m")
    secret = "tok-9f3a$x y"
    text = flow("set S = ${env:HF_TEST_VAL}", "task a", py("open('%s','w').write('1')" % m),
                "task t after a", "  run %s %s ${S} \"${env:HF_TEST_VAL}\" ${env:HF_TEST_UNSET}x" % (PY, dump))
    env = {"HF_TEST_VAL": secret, "HF_TEST_UNSET": None}
    r = runff(c, text, ("run", "t"), grants=join(PX, "environment"), env=env)
    rc_is(r, 0)
    check(r.out.splitlines()[3] == "| " + hexargs(secret, secret, "x"), "expansion: %r" % r.out, r)
    os.remove(m)
    rp = os.path.join(d, "r.json")
    r = runff(c, text, ("run", "t", "--report", rp), grants=join(PX, rw(d)), env=env)
    clean_denial(r, T)
    check(r.err == "taskflow: capability denied: environment\n", "stderr: %r" % r.err, r)
    check(r.out == "" and not os.path.exists(m) and not os.path.exists(rp), "nothing executed or written", r)
    check(secret not in r.out + r.err, "secret leaked", r)
    rc_is(runff(c, text, ("plan",), grants="", extra=["t"], env=env), 0)       # never touch the environment
    rc_is(runff(c, text, ("list",), grants="", env=env), 0)


# ------------------------------------------------------------------ R13 filesystem
@test("T-R13-fs-denied", ["R13", "R9", "R10"], "Flowfile/report/history access is governed; denials leave no side effects")
def _(c):
    d = c.tmp()
    other = c.tmp()
    m = os.path.join(d, "m")
    ffp = put(d, "Flowfile", flow("task t", py("open('%s','w').write('1')" % m)))
    clean_denial(c.run(PX, ["run", "-f", ffp, "t"]), T)
    clean_denial(c.run(PX, ["list", "-f", ffp]), T)
    clean_denial(c.run(rd(other), ["plan", "-f", ffp]), T)
    clean_denial(c.run(PX, ["history", os.path.join(d, "h.json")]), T)
    check(not os.path.exists(m), "nothing may run after a read denial")
    # report/history writes denied BEFORE any command runs
    for opt in ("--report", "--history"):
        out = os.path.join(d, "out.json")
        r = c.run(join(PX, rd(d)), ["run", "-f", ffp, opt, out, "t"])
        clean_denial(r, T)
        check(not os.path.exists(m) and not os.path.exists(out), "%s denial must precede execution and leave no file" % opt, r)
    # write-only grant cannot read an existing history; read-only cannot write it
    hp = put(d, "h.json", json.dumps({"version": 1, "runs": []}))
    before = open(hp).read()
    r = c.run(join(PX, rd(d)), ["run", "-f", ffp, "--history", hp, "t"])
    clean_denial(r, T)
    r = runff(c, flow("task t", py("open('%s','w').write('1')" % m)), ("run", "--history", hp, "t"), grants=join(PX, "filesystem:write=" + d))
    clean_denial(r, T)
    check(open(hp).read() == before and not os.path.exists(m), "history changed / command ran after denial", r)
    # report path outside the write root
    r = runff(c, flow("task t", py("open('%s','w').write('1')" % m)), ("run", "--report", os.path.join(other, "r.json"), "t"), grants=join(PX, rw(d)))
    clean_denial(r, T)
    check(not os.path.exists(m), "executed although the report path was outside the write root", r)
    # a command's own file effects are not governed by the filesystem grants (children are separate processes)
    r = runff(c, flow("task t", py("open('%s','w').write('1')" % m)), ("run", "t"), grants=PX)
    rc_is(r, 0)
    check(os.path.exists(m), "exec'd child must be able to run", r)


@test("T-R13-minimal-grants", ["R13", "R11"], "works with exactly: read dir + write dir + exec grant (no network/database/coarse process)")
def _(c):
    d = c.tmp()
    p = put(d, "Flowfile", flow("task default", py("print('hi')")))
    r = c.run("filesystem:read=%s,filesystem:write=%s,process:exec=%s -c *" % (d, d, PY),
              ["run", "-f", p, "--report", os.path.join(d, "r.json"), "--history", os.path.join(d, "h.json")], cwd=d)
    rc_is(r, 0)
    check(r.out == blk("default", ["hi"]) + summary(ok=1), "output", r)


# ------------------------------------------------------------------ R14 limits
@test("T-R14-exec-output-limit", ["R14", "R7"], "runner exec-output limit => FAILED (output limit exceeded), run continues per policy, oversize output not printed")
def _(c):
    text = flow("task big", py("print('x' * 5000)"), "task small", py("print('ok')"))
    r = runff(c, text, ("run", "--keep-going", "big", "small"), flags=BASE_FLAGS + " --max-exec-output-bytes 1000")
    rc_is(r, 1)
    check(r.out == "==> big\n<== big FAILED (output limit exceeded)\n" + blk("small", ["ok"]) + summary(ok=1, failed=1), "limit output: %r" % r.out[:400], r)
    check("xxxx" not in r.out, "oversize output printed", r)
    rc_is(runff(c, text, ("run", "big")), 0)      # without the tiny limit it is fine


@test("T-R14-deadline", ["R14"], "runner deadline stops a run with a running child promptly")
def _(c):
    d = c.tmp()
    m = os.path.join(d, "after")
    text = flow("task slow", py("import time; time.sleep(30)"), "task later after slow", py("open('%s','w').write('x')" % m))
    rc_is(runff(c, flow("task fast", py("print(1)")), ("run", "fast"), flags=BASE_FLAGS + " --deadline 20s"), 0)   # positive control
    t = time.time()
    r = runff(c, text, ("run", "later"), flags=BASE_FLAGS + " --deadline 1s", timeout=90)
    check(not r.timed_out and time.time() - t < 20, "deadline not honored (hang)", r)
    check(r.rc not in (0, None), "must fail when the deadline expires", r)
    check(not os.path.exists(m), "later task must not run", r)


@test("T-R14-instruction-limit", ["R14"], "tiny instruction ceiling stops the program non-zero without success output")
def _(c):
    text = fx("Flowfile.basic")
    rc_is(runff(c, text, ("list",), grants=""), 0)     # positive control
    r = runff(c, text, ("list",), grants="", flags="--max-instructions 300", timeout=60)
    check(not r.timed_out and r.rc not in (0, None), "must stop under a tiny ceiling", r)


# ------------------------------------------------------------------ R15 hygiene / determinism
@test("T-R15-determinism-and-hygiene", ["R15", "R6"], "identical stdout across runs; stderr silent on success; no secret in report/history/stderr")
def _(c):
    d = c.tmp()
    secret = "SEKRET-4711"
    text = flow("task t", "  run %s -c pass ${env:HF_TEST_VAL}" % PY, "task u after t", py("print('u')"))
    g = join(PX, rw(d), "environment")
    env = {"HF_TEST_VAL": secret}
    a = runff(c, text, ("run", "--report", os.path.join(d, "r1.json"), "--history", os.path.join(d, "h.json"), "u"), grants=g, env=env)
    b = runff(c, text, ("run", "--report", os.path.join(d, "r2.json"), "--history", os.path.join(d, "h.json"), "u"), grants=g, env=env)
    rc_is(a, 0)
    check(a.out == b.out and a.out == blk("t") + blk("u", ["u"]) + summary(ok=2), "stdout differs between runs", a)
    check(a.err == "" and b.err == "", "stderr must be empty on success", a)
    for f in ("r1.json", "r2.json", "h.json"):
        check(secret not in open(os.path.join(d, f)).read(), "%s leaks the environment value" % f, a)
    check(secret not in a.out + a.err, "stdout/stderr leak the environment value", a)


if __name__ == "__main__":
    sys.exit(main(fixtures_dir=FIX))
