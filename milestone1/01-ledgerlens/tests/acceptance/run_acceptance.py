#!/usr/bin/env python3
"""Black-box acceptance suite for ledgerlens (milestone1/01-ledgerlens).

Usage:  run_acceptance.py LAUNCHER        run all tests
        run_acceptance.py --list          list test ids (no launcher needed)
        run_acceptance.py LAUNCHER --only L-R9 -v

Launcher contract: LAUNCHER GRANTS [APP_ARG...]   (REQUIREMENTS.md 1.2)
Python 3 standard library only.  Implementation independent.
"""
import json
import os
import random
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from harness import (test, check, rc_is, no_runtime_leak, clean_denial, main)  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
FIX = os.path.join(HERE, "fixtures")
T = "ledgerlens"


def fx(name):
    with open(os.path.join(FIX, name), encoding="utf-8", newline="") as f:
        return f.read()


def put(d, name, text):
    p = os.path.join(d, name)
    with open(p, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    return p


def rd(d):
    return "filesystem:read=" + d


def rw(d):
    return "filesystem:read=%s,filesystem:write=%s" % (d, d)


def fail_clean(res, rc, tool=T):
    rc_is(res, rc)
    check(res.out == "", "stdout must be empty on failure", res)
    check(res.err.startswith(tool + ": "), "stderr must start with '%s: '" % tool, res)
    for ln in res.err.splitlines():
        check(ln.startswith(tool + ": "), "every stderr line must be prefixed: %r" % ln, res)
    no_runtime_leak(res)


def csv(*rows, header="date,description,amount", eol="\n"):
    return header + eol + "".join(r + eol for r in rows)


def cat_lines(res):
    return res.out.splitlines()


# ---------------------------------------------------------------- R1 CLI shell
@test("L-R1-version", ["R1"], "version prints fixed string, needs no grants")
def _(c):
    r = c.run("", ["version"])
    rc_is(r, 0)
    check(r.out == "ledgerlens 1.0.0\n", "bad version output", r)


@test("L-R1-help", ["R1"], "help/--help/-h print usage to stdout, exit 0")
def _(c):
    for a in ("help", "--help", "-h"):
        r = c.run("", [a])
        rc_is(r, 0)
        check(r.out.startswith("usage: ledgerlens"), "usage text must start with 'usage: ledgerlens'", r)


@test("L-R1-usage-errors", ["R1", "R18"], "usage errors exit 2, nothing on stdout, no output file")
def _(c):
    d = c.tmp()
    csvp = put(d, "t.csv", csv("2026-01-01,a,-1.00"))
    rules = put(d, "r.ini", "[categories]\nA = a\n")
    out = os.path.join(d, "o.txt")
    bad = [
        [], ["bogus"], ["summary"], ["summary", csvp, csvp],
        ["summary", "--bogus", "x", csvp],
        ["summary", "--format=json", csvp],
        ["summary", "--format", "xml", csvp],
        ["summary", "--format", "json", "--format", "text", csvp],
        ["summary", "--rules", rules, "--rules", rules, csvp],
        ["summary", csvp, "--rules"],
        ["summary", "--max-rows", "0", csvp], ["summary", "--max-rows", "abc", csvp],
        ["summary", "--max-rows", "-5", csvp], ["summary", "--max-rows", "10000001", csvp],
        ["summary", "--month", "2026-13", csvp], ["summary", "--month", "26-01", csvp],
        ["categorize", csvp], ["categorize", "--rules", rules, "--month", "2026-01", csvp],
        ["categorize", "--rules", rules, "--format", "json", csvp],
        ["check-rules"], ["check-rules", rules, rules],
        ["version", "extra"],
    ]
    for a in bad:
        if "--out" not in a:
            a = a + ["--out", out] if a[:1] in (["summary"], ["categorize"]) else a
        r = c.run(rw(d), a)
        rc_is(r, 2)
        check(r.out == "", "usage error wrote to stdout: %r" % a, r)
        check(r.err.startswith(T + ": "), "diagnostic prefix: %r" % a, r)
        check(not os.path.exists(out), "usage error created output file: %r" % a, r)


@test("L-R1-option-order", ["R1"], "options may precede or follow INPUT")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", fx("txns_basic.csv"))
    rules = put(d, "r.ini", fx("rules_basic.ini"))
    a = c.run(rd(d), ["summary", "--rules", rules, "--format", "json", p])
    b = c.run(rd(d), ["summary", p, "--format", "json", "--rules", rules])
    check(a.rc == b.rc == 1 and a.out == b.out and a.out != "", "option order changes behavior", a)


# ---------------------------------------------------------------- R2 CSV parsing
@test("L-R2-basic-golden", ["R2", "R6", "R7", "R8", "R9", "R11", "R17"], "golden summary text for basic fixture, exit 1 (budget over)")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", fx("txns_basic.csv"))
    rules = put(d, "r.ini", fx("rules_basic.ini"))
    r = c.run(rd(d), ["summary", "--rules", rules, p])
    rc_is(r, 1)
    check(r.out == fx("txns_basic.expected.txt"), "summary text differs from golden", r)
    check(r.err == "", "stderr must be empty on a normal run", r)


@test("L-R2-header-variants", ["R2"], "header case/order/extra columns/BOM/CRLF/no trailing newline/blank lines")
def _(c):
    d = c.tmp()
    rules = put(d, "r.ini", "[categories]\nX = x\n")
    variants = [
        "﻿AMOUNT,Date,extra,DESCRIPTION\r\n-1.50,2026-01-02,q,x one\r\n\r\n3.00,2026-01-03,w,y\r\n",
        "amount , description,date\n-1.50,x one,2026-01-02\n\n\n3.00,y,2026-01-03",
        "Description,Date,Amount,Date2,Amount2\n" "x one,2026-01-02,-1.50,zz,zz\n" "y,2026-01-03,3.00,,\n",
    ]
    for v in variants:
        p = put(d, "t.csv", v)
        r = c.run(rd(d), ["categorize", "--rules", rules, p])
        rc_is(r, 0)
        check(r.out == "date,description,amount,category\n2026-01-02,x one,-1.50,X\n2026-01-03,y,3.00,Uncategorized\n",
              "variant parsed wrongly: %r" % v, r)


@test("L-R2-quoting-and-trim", ["R2", "R3", "R12"], "quoted fields, doubled quotes, trimming, extra columns, signs, 12 digits")
def _(c):
    d = c.tmp()
    rules = put(d, "r.ini", fx("rules_basic.ini"))
    p = put(d, "t.csv", fx("txns_messy.csv"))
    r = c.run(rd(d), ["categorize", "--rules", rules, "--lenient", p])
    rc_is(r, 0)
    check(r.out == fx("txns_messy.lenient.categorized.csv"), "categorize output differs", r)
    check(r.err == T + ": skipped 1 invalid row(s)\n", "expected exactly the skipped-row notice", r)


@test("L-R2-utf8", ["R2"], "UTF-8 descriptions pass through byte-exact")
def _(c):
    d = c.tmp()
    rules = put(d, "r.ini", "[categories]\nCafe = café\n")
    p = put(d, "t.csv", fx("txns_utf8.csv"))
    r = c.run(rd(d), ["categorize", "--rules", rules, p])
    rc_is(r, 0)
    check(r.out == "date,description,amount,category\n2026-01-02,café €5 日本,-5.00,Cafe\n", "utf8 mangled", r)


@test("L-R2-structure-errors", ["R2", "R4"], "malformed headers/rows abort with exit 65 and physical line numbers")
def _(c):
    d = c.tmp()
    cases = [
        ("", None, "empty"),
        ("\n\n", None, "empty"),
        ("date,amount\n2026-01-01,1\n", 1, "description"),
        ("date,description,amount,Amount\n2026-01-01,a,1,1\n", 1, "amount"),
        ('date,description,amount\n\n2026-01-01,"abc,5\n', 3, ""),
        ('date,description,amount\n2026-01-01,"abc"x,5\n', 2, ""),
        ('date,description,amount\n2026-01-01,ab"c,5\n', 2, ""),
        ("date,description,amount\n2026-01-01,a,5,6\n", 2, ""),
        ("date,description,amount\n2026-01-01,a\n", 2, ""),
        ("date,description,amount\n   \n", 2, ""),
    ]
    for text, line, _w in cases:
        p = put(d, "t.csv", text)
        r = c.run(rd(d), ["summary", p])
        fail_clean(r, 65)
        if line is not None:
            check(("line %d" % line) in r.err, "expected 'line %d' in diagnostic for %r" % (line, text), r)


# ---------------------------------------------------------------- R3 validation
@test("L-R3-invalid-values", ["R3", "R4"], "each invalid date/amount aborts with exit 65 naming the line")
def _(c):
    d = c.tmp()
    bad = ["2026-13-01,x,1", "2026-00-10,x,1", "2026-04-31,x,1", "2023-02-29,x,1", "26-01-01,x,1",
           "2026-1-5,x,1", "1899-12-31,x,1", "3000-01-01,x,1", "2026-01-01,x,.5", "2026-01-01,x,5.",
           "2026-01-01,x,1e3", '2026-01-01,x,"1,000.00"', "2026-01-01,x,$5", "2026-01-01,x,1.234",
           "2026-01-01,x,--5", "2026-01-01,x,", "2026-01-01,x,abc", "2026-01-01,x,1234567890123",
           "2026-01-01 ,x,1x", ",x,1", "2026-01-01," + "a" * 1001 + ",1"]
    for row in bad:
        p = put(d, "t.csv", csv("2026-01-01,ok,1.00", row))
        r = c.run(rd(d), ["summary", p])
        fail_clean(r, 65)
        check("line 3" in r.err, "diagnostic must name 'line 3' for %r" % row, r)


@test("L-R3-valid-values", ["R3", "R12"], "boundary-valid dates/amounts accepted and canonicalized")
def _(c):
    d = c.tmp()
    rules = put(d, "r.ini", "[categories]\nX = x\n")
    rows = ["2024-02-29,x,+0", "2026-01-01,x,-0.00", "1900-01-01,x,12.5", "2999-12-31,x, 7 ",
            "2000-02-29,x,-999999999999.99", "2026-06-30,x,0.1", "2026-07-31,x,-0", "2026-01-01," + "a" * 1000 + ",1"]
    p = put(d, "t.csv", csv(*rows))
    r = c.run(rd(d), ["categorize", "--rules", rules, p])
    rc_is(r, 0)
    amts = [ln.split(",")[2] for ln in cat_lines(r)[1:8]]
    check(amts == ["0.00", "0.00", "12.50", "7.00", "-999999999999.99", "0.10", "0.00"], "amount canonicalization: %r" % amts, r)


@test("L-R3-exact-cents", ["R3", "R7"], "sums are exact (no float drift), large values exact")
def _(c):
    d = c.tmp()
    rows = ["2026-01-%02d,x,-0.10" % (1 + i % 28) for i in range(300)] + ["2026-01-05,y,999999999999.99",
                                                                            "2026-01-06,y,999999999999.99"]
    p = put(d, "t.csv", csv(*rows))
    r = c.run(rd(d), ["summary", "--format", "json", p])
    rc_is(r, 0)
    j = json.loads(r.out)
    check(j["total"] == {"income": "1999999999999.98", "expenses": "30.00", "net": "1999999999969.98", "count": 302},
          "exact arithmetic failed: %r" % j["total"], r)


# ---------------------------------------------------------------- R4 error policy
@test("L-R4-strict-no-side-effects", ["R4", "R14"], "strict failure: empty stdout, no --out file, diagnostic names line")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", csv("2026-01-01,a,-1.00", "2026-02-30,b,-2.00"))
    out = os.path.join(d, "o.txt")
    r = c.run(rw(d), ["summary", "--out", out, p])
    fail_clean(r, 65)
    check("line 3" in r.err, "must name line 3", r)
    check(not os.path.exists(out), "output file must not exist", r)
    put(d, "o.txt", "PRECIOUS")
    r = c.run(rw(d), ["summary", "--out", out, p])
    rc_is(r, 65)
    check(open(out).read() == "PRECIOUS", "pre-existing output file modified on failure", r)


@test("L-R4-lenient", ["R4"], "--lenient skips invalid rows, counts them, header errors stay fatal")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", csv("2026-01-01,a,-1.00", "bad,b,-2.00", '2026-01-02,"q,5', "2026-01-03,c,3.00,9", "2026-01-04,d,x"))
    r = c.run(rd(d), ["summary", "--lenient", "--format", "json", p])
    rc_is(r, 0)
    j = json.loads(r.out)
    check(j["transactions"] == 1 and j["skipped"] == 4, "counts: %r" % j, r)
    check(r.err == T + ": skipped 4 invalid row(s)\n", "skip notice", r)
    r = c.run(rd(d), ["summary", "--lenient", "--format", "json", put(d, "h.csv", "date,amount\n2026-01-01,1\n")])
    fail_clean(r, 65)
    r = c.run(rd(d), ["summary", "--lenient", put(d, "e.csv", "")])
    fail_clean(r, 65)
    check("empty input" in r.err, "expected 'empty input'", r)
    r = c.run(rd(d), ["summary", "--lenient", "--format", "json", put(d, "ok.csv", csv("2026-01-01,a,-1.00"))])
    rc_is(r, 0)
    check(r.err == "", "no notice when nothing skipped", r)
    check(json.loads(r.out)["skipped"] == 0, "skipped must be 0", r)


# ---------------------------------------------------------------- R5 rules
@test("L-R5-check-rules-ok", ["R5"], "check-rules OK line")
def _(c):
    d = c.tmp()
    r = c.run(rd(d), ["check-rules", put(d, "r.ini", fx("rules_basic.ini"))])
    rc_is(r, 0)
    check(r.out == "OK: 4 rule(s), 3 budget(s)\n", "check-rules output", r)
    r = c.run(rd(d), ["check-rules", put(d, "e.ini", "# nothing\n\n")])
    rc_is(r, 0)
    check(r.out == "OK: 0 rule(s), 0 budget(s)\n", "empty rules", r)
    r = c.run(rd(d), ["check-rules", put(d, "m.ini", "[categories]\nA = x=y\nA = z\r\n[budgets]\nA = 5.5\n[categories]\nB = (?i)b\n[budgets]\nUncategorized = 1\n")])
    rc_is(r, 0)
    check(r.out == "OK: 3 rule(s), 2 budget(s)\n", "multi-section rules", r)


@test("L-R5-check-rules-errors", ["R5"], "rules errors name the physical line, exit 65")
def _(c):
    d = c.tmp()
    cases = [
        ("A = x\n", 1), ("[categories]\nbad line\n", 2), ("[categories]\n1A = x\n", 2),
        ("[categories]\nUncategorized = x\n", 2), ("[categories]\nA = \n", 2), ("[categories]\nA = (\n", 2),
        ("[categories]\n# c\n\nA = x\n[budgets]\nB = 5\n", 6), ("[categories]\nA = x\n[budgets]\nA = 0\n", 4),
        ("[categories]\nA = x\n[budgets]\nA = 1.234\n", 4), ("[categories]\nA = x\n[budgets]\nA = 1\nA = 2\n", 5),
        ("[categories]\nA = x\n[budgets]\nA = -3\n", 4), ("[Categories]\nA = x\n", 1), ("[other]\n", 1),
        ("[categories]\n" + "N" * 41 + " = x\n", 2),
    ]
    for text, line in cases:
        p = put(d, "r.ini", text)
        r = c.run(rd(d), ["check-rules", p])
        fail_clean(r, 65)
        check(("rules: line %d" % line) in r.err, "expected 'rules: line %d' for %r" % (line, text), r)
    # the same validation applies when rules are used by summary
    p = put(d, "r.ini", "[categories]\nA = (\n")
    r = c.run(rd(d), ["summary", "--rules", p, put(d, "t.csv", csv("2026-01-01,a,1"))])
    fail_clean(r, 65)


# ---------------------------------------------------------------- R6 categorization
@test("L-R6-priority-and-regex", ["R6", "R12"], "first matching rule wins, unanchored, case-sensitive, (?i)")
def _(c):
    d = c.tmp()
    rules = put(d, "r.ini", "[categories]\nFirst = shop\nSecond = shop|bar\nThird = ^bar\nCI = (?i)LOUD\nEq = a=b\n")
    rows = ["2026-01-01,corner shop,-1", "2026-01-01,wine bar,-1", "2026-01-01,bar none,-1", "2026-01-01,Shop,-1",
            "2026-01-01,loud NOISE,-1", "2026-01-01,LOUD,-1", "2026-01-01,x a=b y,-1", "2026-01-01,,-1"]
    r = c.run(rd(d), ["categorize", "--rules", rules, put(d, "t.csv", csv(*rows))])
    rc_is(r, 0)
    cats = [ln.rsplit(",", 1)[1] for ln in cat_lines(r)[1:]]
    check(cats == ["First", "Second", "Second", "Uncategorized", "CI", "CI", "Eq", "Uncategorized"], "categories: %r" % cats, r)


@test("L-R6-no-rules", ["R6", "R7", "R8"], "summary without --rules: everything Uncategorized, no budgets section")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", csv("2026-01-01,a,-5.00", "2026-01-02,b,2.00"))
    r = c.run(rd(d), ["summary", p])
    rc_is(r, 0)
    check(r.out == "# monthly\nmonth\tincome\texpenses\tnet\ttransactions\n2026-01\t2.00\t5.00\t-3.00\t2\nTOTAL\t2.00\t5.00\t-3.00\t2\n\n"
                   "# categories\nmonth\tcategory\tincome\texpenses\ttransactions\n2026-01\tUncategorized\t2.00\t5.00\t2\n", "no-rules output", r)


# ---------------------------------------------------------------- R7-R11 summary
@test("L-R8-category-order", ["R8"], "category rows: month, expenses desc, name asc")
def _(c):
    d = c.tmp()
    rules = put(d, "r.ini", "[categories]\nBeta = b\nAlpha = a\nGamma = g\nZed = z\n")
    rows = ["2026-02-01,a,-5", "2026-01-01,b,-5", "2026-01-02,a,-5", "2026-01-03,g,-9.99", "2026-01-04,z,100", "2026-01-05,q,-5"]
    r = c.run(rd(d), ["summary", "--rules", rules, "--format", "json", put(d, "t.csv", csv(*rows))])
    rc_is(r, 0)
    got = [(x["month"], x["category"], x["expenses"]) for x in json.loads(r.out)["categories"]]
    check(got == [("2026-01", "Gamma", "9.99"), ("2026-01", "Alpha", "5.00"), ("2026-01", "Beta", "5.00"),
                  ("2026-01", "Uncategorized", "5.00"), ("2026-01", "Zed", "0.00"), ("2026-02", "Alpha", "5.00")],
          "category order: %r" % got, r)


@test("L-R9-budget-semantics", ["R9"], "equal is OK, refunds do not reduce spend, Uncategorized budget, zero-spend rows")
def _(c):
    d = c.tmp()
    rules = put(d, "r.ini", "[categories]\nFood = food\n[budgets]\nUncategorized = 5\nFood = 10.00\n")
    rows = ["2026-01-01,food,-10.00", "2026-01-02,food,4.00", "2026-01-03,other,-5.01", "2026-02-01,other,100"]
    r = c.run(rd(d), ["summary", "--rules", rules, "--format", "json", put(d, "t.csv", csv(*rows))])
    rc_is(r, 1)
    b = json.loads(r.out)["budgets"]
    check([(x["month"], x["category"], x["limit"], x["spent"], x["status"]) for x in b] == [
        ("2026-01", "Food", "10.00", "10.00", "OK"), ("2026-01", "Uncategorized", "5.00", "5.01", "OVER"),
        ("2026-02", "Food", "10.00", "0.00", "OK"), ("2026-02", "Uncategorized", "5.00", "0.00", "OK")], "budget rows: %r" % b, r)
    rules2 = put(d, "r2.ini", "[categories]\nFood = food\n[budgets]\nFood = 10.00\n")
    r = c.run(rd(d), ["summary", "--rules", rules2, put(d, "t2.csv", csv("2026-01-01,food,-10.00"))])
    rc_is(r, 0)


@test("L-R10-month-filter", ["R10"], "--month filters; no match gives empty report; bad month is usage error")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", fx("txns_basic.csv"))
    rules = put(d, "r.ini", fx("rules_basic.ini"))
    r = c.run(rd(d), ["summary", "--rules", rules, "--month", "2026-01", "--format", "json", p])
    rc_is(r, 0)
    j = json.loads(r.out)
    check([m["month"] for m in j["months"]] == ["2026-01"] and j["transactions"] == 7
          and j["total"]["expenses"] == "1310.75" and all(b["month"] == "2026-01" for b in j["budgets"]), "filter result", r)
    r = c.run(rd(d), ["summary", "--rules", rules, "--month", "2030-05", "--format", "json", p])
    rc_is(r, 0)
    j = json.loads(r.out)
    check(j["months"] == [] and j["categories"] == [] and j["budgets"] == [] and j["transactions"] == 0
          and j["total"] == {"income": "0.00", "expenses": "0.00", "net": "0.00", "count": 0}, "empty filter result", r)
    r = c.run(rd(d), ["summary", "--month", "2030-05", p])
    rc_is(r, 0)
    check(r.out == "# monthly\nmonth\tincome\texpenses\tnet\ttransactions\nTOTAL\t0.00\t0.00\t0.00\t0\n\n# categories\nmonth\tcategory\tincome\texpenses\ttransactions\n", "empty text report", r)


@test("L-R11-json-golden", ["R11"], "JSON document shape and values for basic fixture")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", fx("txns_basic.csv"))
    rules = put(d, "r.ini", fx("rules_basic.ini"))
    r = c.run(rd(d), ["summary", "--rules", rules, "--format", "json", p])
    rc_is(r, 1)
    check(r.out.endswith("\n"), "JSON must end with newline", r)
    j = json.loads(r.out)
    check(set(j) == {"transactions", "skipped", "months", "total", "categories", "budgets"}, "keys: %r" % sorted(j), r)
    check(j["transactions"] == 11 and j["skipped"] == 0, "counts", r)
    check(j["months"][0] == {"month": "2026-01", "income": "3020.00", "expenses": "1310.75", "net": "1709.25", "count": 7}, "month row", r)
    check(j["total"] == {"income": "6020.00", "expenses": "2670.75", "net": "3349.25", "count": 11}, "total", r)
    check(j["categories"][1] == {"month": "2026-01", "category": "Groceries", "income": "20.00", "expenses": "270.50", "count": 3}, "category row", r)
    check(j["budgets"][4] == {"month": "2026-02", "category": "Groceries", "limit": "300.00", "spent": "310.00", "status": "OVER"}, "budget row", r)
    check(len(j["categories"]) == 9 and len(j["budgets"]) == 6, "row counts", r)


@test("L-R7-negative-net-format", ["R7"], "negative net formatting, income-only month, no '-0.00'")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", csv("2026-01-01,a,-12.30", "2026-02-01,b,5", "2026-03-01,c,-0.00", "2026-03-02,c,0"))
    r = c.run(rd(d), ["summary", "--format", "json", p])
    rc_is(r, 0)
    m = {x["month"]: x for x in json.loads(r.out)["months"]}
    check(m["2026-01"]["net"] == "-12.30" and m["2026-02"]["net"] == "5.00" and m["2026-03"]["net"] == "0.00"
          and m["2026-03"]["count"] == 2, "formatting: %r" % m, r)
    check("-0.00" not in r.out, "-0.00 must never be printed", r)


# ---------------------------------------------------------------- R12 categorize (golden)
@test("L-R12-categorize-golden", ["R12", "R6"], "categorize golden output")
def _(c):
    d = c.tmp()
    r = c.run(rd(d), ["categorize", "--rules", put(d, "r.ini", fx("rules_basic.ini")), put(d, "t.csv", fx("txns_basic.csv"))])
    rc_is(r, 0)
    check(r.out == fx("txns_basic.categorized.csv"), "categorized csv differs", r)


@test("L-R12-quoting-rules", ["R12"], "category/description quoting only when needed")
def _(c):
    d = c.tmp()
    rules = put(d, "r.ini", "[categories]\nSpace Name = sp\n")
    p = put(d, "t.csv", csv('2026-01-01," sp lead",-1', '2026-01-01,"say ""hi""",-1', '2026-01-01,"trail ",-1', "2026-01-01,plain,+3.5"))
    r = c.run(rd(d), ["categorize", "--rules", rules, p])
    rc_is(r, 0)
    check(cat_lines(r)[1:] == ['2026-01-01," sp lead",-1.00,Space Name', '2026-01-01,"say ""hi""",-1.00,Uncategorized',
                               '2026-01-01,"trail ",-1.00,Uncategorized', "2026-01-01,plain,3.50,Uncategorized"], "quoting", r)


# ---------------------------------------------------------------- R13 stdin
@test("L-R13-stdin", ["R13", "R16"], "INPUT '-' reads stdin; no grants needed without --rules")
def _(c):
    r = c.run("", ["summary", "--format", "json", "-"], stdin=csv("2026-01-01,a,-1.00", "2026-01-02,b,2.00"))
    rc_is(r, 0)
    check(json.loads(r.out)["total"]["net"] == "1.00", "stdin summary", r)
    d = c.tmp()
    rules = put(d, "r.ini", fx("rules_basic.ini"))
    r = c.run(rd(d), ["categorize", "--rules", rules, "-"], stdin=fx("txns_basic.csv"))
    rc_is(r, 0)
    check(r.out == fx("txns_basic.categorized.csv"), "stdin categorize", r)
    r = c.run("", ["summary", "-"], stdin="")
    fail_clean(r, 65)
    r = c.run(rd(d), ["summary", "-"], stdin=b"\xef\xbb\xbfdate,description,amount\r\n2026-01-01,a,1\r\n")
    rc_is(r, 0)


# ---------------------------------------------------------------- R14 --out
@test("L-R14-out-file", ["R14", "R9"], "--out writes the stdout bytes, keeps exit code, stdout empty")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", fx("txns_basic.csv"))
    rules = put(d, "r.ini", fx("rules_basic.ini"))
    out = os.path.join(d, "report.txt")
    put(d, "report.txt", "old old old old old old old old old old old old old old old old")
    r = c.run(rw(d), ["summary", "--rules", rules, "--out", out, p])
    rc_is(r, 1)
    check(r.out == "", "stdout must be empty with --out", r)
    check(open(out, encoding="utf-8", newline="").read() == fx("txns_basic.expected.txt"), "file content", r)
    out2 = os.path.join(d, "cat.csv")
    r = c.run(rw(d), ["categorize", "--rules", rules, "--out", out2, p])
    rc_is(r, 0)
    check(open(out2, encoding="utf-8", newline="").read() == fx("txns_basic.categorized.csv"), "categorize file", r)
    r = c.run(rw(d), ["summary", "--format", "json", "--out", os.path.join(d, "nodir", "x.json"), p])
    fail_clean(r, 73)
    check("cannot write" in r.err, "expected 'cannot write'", r)


# ---------------------------------------------------------------- R15 limits
@test("L-R15-max-rows", ["R15"], "--max-rows limit exit 70, empty stdout, no file")
def _(c):
    d = c.tmp()
    rows = ["2026-01-%02d,a,-1" % (1 + i % 28) for i in range(10)]
    p = put(d, "t.csv", csv(*rows))
    out = os.path.join(d, "o.txt")
    r = c.run(rw(d), ["summary", "--max-rows", "9", "--out", out, p])
    fail_clean(r, 70)
    check("row limit exceeded (max 9)" in r.err, "message", r)
    check(not os.path.exists(out), "no output file", r)
    r = c.run(rd(d), ["summary", "--max-rows", "10", p])
    rc_is(r, 0)
    # invalid rows count toward the limit too; blank lines do not
    p2 = put(d, "t2.csv", csv("2026-01-01,a,-1", "bad", "", "2026-01-01,b,-1"))
    rc_is(c.run(rd(d), ["summary", "--max-rows", "3", p2, "--lenient"]), 0)
    fail_clean(c.run(rd(d), ["summary", "--max-rows", "2", p2, "--lenient"]), 70)


@test("L-R15-runtime-limits", ["R15"], "tiny instruction/memory ceilings stop the run promptly with no partial report")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", csv(*["2026-01-%02d,item %d,-%d.25" % (1 + i % 28, i, i % 50) for i in range(400)]))
    rc_is(c.run(rd(d), ["summary", p]), 0)   # positive control: same input succeeds under the generous ceilings
    for flags in ("--max-instructions 2000", "--max-memory-bytes 20000"):
        r = c.run(rd(d), ["summary", p], flags=flags, timeout=60)
        check(not r.timed_out, "hang under limit", r)
        check(r.rc not in (0, None), "must fail under tiny limit", r)
        check(r.out == "", "no partial report may be printed", r)


# ---------------------------------------------------------------- R16 capabilities
@test("L-R16-read-denied", ["R16", "R18"], "no grants: reading input/rules file is a clean denial (77)")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", fx("txns_basic.csv"))
    rules = put(d, "r.ini", fx("rules_basic.ini"))
    for args in (["summary", p], ["categorize", "--rules", rules, "-"], ["check-rules", rules], ["summary", "--rules", rules, "-"]):
        r = c.run("", args, stdin=fx("txns_basic.csv"))
        clean_denial(r, T)
        check(r.out == "", "stdout must be empty on denial", r)
    r = c.run("", ["summary", os.path.join(d, "does-not-exist.csv")])
    clean_denial(r, T)   # denial even though the file does not exist


@test("L-R16-scope", ["R16"], "read grant is scoped to its root; paths outside are denied; missing inside is 66")
def _(c):
    d = c.tmp()
    other = c.tmp()
    sub = os.path.join(d, "in")
    os.makedirs(sub)
    p_in = put(sub, "t.csv", fx("txns_basic.csv"))
    p_out = put(other, "t.csv", fx("txns_basic.csv"))
    rc_is(c.run(rd(sub), ["summary", p_in]), 0)
    clean_denial(c.run(rd(sub), ["summary", p_out]), T)
    clean_denial(c.run(rd(sub), ["summary", os.path.join(sub, "..", "..", os.path.basename(other), "t.csv")]), T)
    r = c.run(rd(sub), ["summary", os.path.join(sub, "missing.csv")])
    fail_clean(r, 66)
    fail_clean(c.run(rd(sub), ["summary", sub]), 66)   # a directory is not a readable regular file
    r = c.run(rd(sub), ["check-rules", os.path.join(sub, "nope.ini")])
    fail_clean(r, 66)
    link = os.path.join(sub, "link.csv")
    os.symlink(p_out, link)
    clean_denial(c.run(rd(sub), ["summary", link]), T)   # symlink escaping the root


@test("L-R16-write-denied", ["R16", "R14"], "read-only grant: --out is denied (77), nothing created; write-only grant cannot read")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", fx("txns_basic.csv"))
    out = os.path.join(d, "o.txt")
    r = c.run(rd(d), ["summary", "--out", out, p])
    clean_denial(r, T)
    check(not os.path.exists(out), "output file must not be created", r)
    put(d, "keep.txt", "KEEP")
    r = c.run(rd(d), ["summary", "--out", os.path.join(d, "keep.txt"), p])
    clean_denial(r, T)
    check(open(os.path.join(d, "keep.txt")).read() == "KEEP", "existing file must be untouched", r)
    # write grant alone: reading input is denied, stdin input + out works
    r = c.run("filesystem:write=" + d, ["summary", p])
    clean_denial(r, T)
    r = c.run("filesystem:write=" + d, ["summary", "--out", out, "-"], stdin=fx("txns_basic.csv"))
    rc_is(r, 0)
    check(os.path.exists(out), "write-only grant must permit --out", r)


@test("L-R16-minimal-grants", ["R16"], "works with ONLY the narrow filesystem grants (no network/process/env/db)")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", fx("txns_basic.csv"))
    rules = put(d, "r.ini", fx("rules_basic.ini"))
    r = c.run(rd(d), ["summary", "--rules", rules, p])
    rc_is(r, 1)
    check(r.out == fx("txns_basic.expected.txt"), "output under minimal grants", r)


# ---------------------------------------------------------------- R17 scale/determinism
def oracle(rows, cat_of, budgets):
    mon, cats = {}, {}
    for date, desc, cents in rows:
        m = date[:7]
        cat = cat_of(desc)
        for tab, k in ((mon, m), (cats, (m, cat))):
            e = tab.setdefault(k, [0, 0, 0])
            if cents > 0:
                e[0] += cents
            else:
                e[1] += -cents
            e[2] += 1
    return mon, cats


def money(v):
    s = "%d.%02d" % (abs(v) // 100, abs(v) % 100)
    return "-" + s if v < 0 else s


@test("L-R17-scale-oracle", ["R17", "R7", "R8", "R9", "R6"], "5,000 generated rows match an independent Python oracle; output deterministic")
def _(c):
    rnd = random.Random(20261013)
    names = ["Food", "Fuel", "Rent", "Fun", "Gym", "Pets", "Kids", "Tax", "Gift", "Misc", "Travel", "Health"]
    pats = {n: n.lower() for n in names[:-1]}
    pats["Travel"] = "(?i)airline|hotel"
    rules_txt = "[categories]\n" + "".join("%s = %s\n" % (n, pats[n]) for n in names[:-1])
    budgets = {"Food": 40000, "Fuel": 20000, "Fun": 15000, "Travel": 100000}
    rules_txt += "[budgets]\n" + "".join("%s = %s\n" % (k, money(v)) for k, v in budgets.items())
    rows, lines = [], []
    words = [n.lower() for n in names[:-1]] + ["AIRLINE x", "Hotel y", "zzz", "unmatched", "FOOD (caps)"]
    for i in range(5000):
        date = "%04d-%02d-%02d" % (2025 + rnd.randrange(2), 1 + rnd.randrange(12), 1 + rnd.randrange(28))
        desc = "%s #%d" % (rnd.choice(words), i)
        cents = rnd.randrange(-30000, 40000)
        rows.append((date, desc, cents))
        lines.append("%s,%s,%s" % (date, desc, money(cents)))

    def cat_of(desc):
        import re
        for n in names[:-1]:
            if re.search(pats[n], desc):
                return n
        return "Uncategorized"
    d = c.tmp()
    rules = put(d, "r.ini", rules_txt)
    p = put(d, "t.csv", csv(*lines))
    r = c.run(rd(d), ["summary", "--rules", rules, "--format", "json", p], timeout=170)
    check(r.rc in (0, 1), "unexpected exit", r)
    j = json.loads(r.out)
    mon, cats = oracle(rows, cat_of, budgets)
    check([m["month"] for m in j["months"]] == sorted(mon), "months", r)
    for m in j["months"]:
        e = mon[m["month"]]
        check((m["income"], m["expenses"], m["net"], m["count"]) == (money(e[0]), money(e[1]), money(e[0] - e[1]), e[2]), "month %s" % m["month"], r)
    exp_cats = sorted(cats.items(), key=lambda kv: (kv[0][0], -kv[1][1], kv[0][1]))
    check([(x["month"], x["category"], x["income"], x["expenses"], x["count"]) for x in j["categories"]] ==
          [(k[0], k[1], money(v[0]), money(v[1]), v[2]) for k, v in exp_cats], "category rows", r)
    over = False
    exp_b = []
    for m in sorted(mon):
        for n in sorted(budgets):
            spent = cats.get((m, n), [0, 0, 0])[1]
            st = "OVER" if spent > budgets[n] else "OK"
            over = over or st == "OVER"
            exp_b.append((m, n, money(budgets[n]), money(spent), st))
    check([(x["month"], x["category"], x["limit"], x["spent"], x["status"]) for x in j["budgets"]] == exp_b, "budget rows", r)
    check(r.rc == (1 if over else 0), "exit code vs budgets", r)
    r2 = c.run(rd(d), ["summary", "--rules", rules, "--format", "json", p], timeout=170)
    check(r2.out == r.out and r2.rc == r.rc, "non-deterministic output", r2)


# ---------------------------------------------------------------- R18 hygiene
@test("L-R18-no-stderr-on-success", ["R18"], "successful runs write nothing to stderr")
def _(c):
    d = c.tmp()
    p = put(d, "t.csv", fx("txns_basic.csv"))
    rules = put(d, "r.ini", fx("rules_basic.ini"))
    for a in (["categorize", "--rules", rules, p], ["check-rules", rules], ["version"], ["summary", "--month", "2026-01", p]):
        r = c.run(rd(d), a)
        rc_is(r, 0)
        check(r.err == "", "stderr not empty: %r" % a, r)


if __name__ == "__main__":
    sys.exit(main(fixtures_dir=FIX))
