"""Held-out task specs (frozen before any reference solution is written).

Each task lists input files, one output path, an output kind and the expected
output. Expected outputs are computed here by a spec-time oracle from the task
text; this file is part of the frozen spec commit and is not a reference
solution (it is never counted or verified as one).

Output comparison (verifier contract):
  text: exact match after removing trailing newlines from both sides.
  json: decoded-value equality.
"""
import json, re
from collections import Counter, OrderedDict
from pathlib import Path

T = []
def task(id, cat, desc, inputs, out, kind, expect):
    T.append({"id": id, "category": cat, "description": desc, "inputs": inputs,
              "output": out, "kind": kind,
              "expected": expect if kind == "text" else expect})

def lines(s): return s.split("\n")

# ---- text transforms ----
txt = "alpha beta\nGamma delta\n\nfoo bar foo\n  padded line  \nlast"
task("h01", "text", "Read in/text.txt. Write out/result.txt with every line upper-cased, same line order.",
     {"in/text.txt": txt}, "out/result.txt", "text", "\n".join(l.upper() for l in lines(txt)))
task("h02", "text", "Read in/text.txt. Write out/result.txt with the lines in reverse order.",
     {"in/text.txt": txt}, "out/result.txt", "text", "\n".join(reversed(lines(txt))))
task("h03", "text", "Read in/text.txt. Write out/result.txt where each line is prefixed with its 1-based line number and \": \" (e.g. \"1: alpha beta\").",
     {"in/text.txt": txt}, "out/result.txt", "text", "\n".join(f"{i+1}: {l}" for i, l in enumerate(lines(txt))))
task("h04", "text", "Read in/text.txt. Trim leading/trailing spaces of every line, drop lines that are then empty, and write the rest to out/result.txt.",
     {"in/text.txt": txt}, "out/result.txt", "text", "\n".join(l.strip() for l in lines(txt) if l.strip()))
task("h05", "text", "Read in/text.txt. Replace every occurrence of the substring \"foo\" with \"bar\" and write the result to out/result.txt.",
     {"in/text.txt": txt}, "out/result.txt", "text", txt.replace("foo", "bar"))

# ---- CSV ----
csv = "name,email,age,dept,amount\nAda,ada@example.com,36,eng,120\nBob,bob@test.org,25,ops,75\nCy,cy@example.com,30,eng,40\nDee,dee@corp.net,41,sales,300\nEd,ed@test.org,29,ops,15"
rows = [dict(zip(lines(csv)[0].split(","), r.split(","))) for r in lines(csv)[1:]]
task("h06", "csv", "Read in/data.csv (header row, comma-separated, no quoting). Write the email column values, one per line in row order, to out/result.txt.",
     {"in/data.csv": csv}, "out/result.txt", "text", "\n".join(r["email"] for r in rows))
task("h07", "csv", "Read in/data.csv. Write out/result.csv containing the header row followed by only the rows whose integer age is >= 30, in input order.",
     {"in/data.csv": csv}, "out/result.csv", "text", "\n".join([lines(csv)[0]] + [l for l, r in zip(lines(csv)[1:], rows) if int(r["age"]) >= 30]))
task("h08", "csv", "Read in/data.csv. Write the integer sum of the amount column to out/result.txt.",
     {"in/data.csv": csv}, "out/result.txt", "text", str(sum(int(r["amount"]) for r in rows)))
task("h09", "csv", "Read in/data.csv. Write out/result.json: a JSON array with one object per row mapping each header name to the row's string value.",
     {"in/data.csv": csv}, "out/result.json", "json", rows)
task("h10", "csv", "Read in/data.csv. Write out/result.json: a JSON object mapping each dept value to the integer number of rows with that dept.",
     {"in/data.csv": csv}, "out/result.json", "json", dict(Counter(r["dept"] for r in rows)))

# ---- aggregation ----
nums = "12\n-4\n7\n30\n0\n9"
ns = [int(x) for x in lines(nums)]
task("h11", "aggregate", "Read in/numbers.txt (one integer per line). Write out/result.json: {\"min\": m, \"max\": M, \"sum\": s} as integers.",
     {"in/numbers.txt": nums}, "out/result.json", "json", {"min": min(ns), "max": max(ns), "sum": sum(ns)})
words = "the cat and the hat\nThe bat and THE cat"
task("h12", "aggregate", "Read in/words.txt. Lower-case it, split every line on single spaces, and write out/result.json mapping each word to its integer count.",
     {"in/words.txt": words}, "out/result.json", "json", dict(Counter(w for l in lines(words.lower()) for w in l.split(" "))))
scores = [88, 92, 79, 65, 100, 71]
task("h13", "aggregate", "Read in/scores.json (a JSON array of integers). Write out/result.txt containing the integer mean rounded down (floor).",
     {"in/scores.json": json.dumps(scores)}, "out/result.txt", "text", str(sum(scores) // len(scores)))
log = "INFO start\nERROR disk full\nWARN slow\nERROR timeout\ninfo ERROR-free\nDEBUG x"
task("h14", "aggregate", "Read in/app.log. Write out/result.txt containing the integer number of lines that contain the substring \"ERROR\" (case-sensitive).",
     {"in/app.log": log}, "out/result.txt", "text", str(sum("ERROR" in l for l in lines(log))))
task("h15", "aggregate", "Read in/text.txt. Write the longest line (by character count; the first one on ties) to out/result.txt.",
     {"in/text.txt": txt}, "out/result.txt", "text", max(lines(txt), key=len))

# ---- string parsing ----
kv = "host=db.local\nport=5432\nuser=admin\nmode=read=only"
task("h16", "parse", "Read in/config.kv (key=value per line; split on the first '=' only). Write out/result.json mapping keys to string values.",
     {"in/config.kv": kv}, "out/result.json", "json", dict(l.split("=", 1) for l in lines(kv)))
plog = "INFO|started\nWARN|disk 80%\nERROR|crash\nINFO|retry\nINFO|ok\nERROR|again"
task("h17", "parse", "Read in/events.log (LEVEL|message per line). Write out/result.json mapping each LEVEL to its integer count.",
     {"in/events.log": plog}, "out/result.json", "json", dict(Counter(l.split("|")[0] for l in lines(plog))))
emails = "ada@example.com\nbob@mail.test.org\ncy@x.io"
task("h18", "parse", "Read in/emails.txt (one address per line). Write the domain part (after '@') of each, one per line, to out/result.txt.",
     {"in/emails.txt": emails}, "out/result.txt", "text", "\n".join(e.split("@")[1] for e in lines(emails)))
times = "01:30\n00:45\n02:05\n10:00"
task("h19", "parse", "Read in/times.txt (HH:MM durations, one per line). Write the integer total number of minutes to out/result.txt.",
     {"in/times.txt": times}, "out/result.txt", "text", str(sum(int(t.split(":")[0]) * 60 + int(t.split(":")[1]) for t in lines(times))))
tags = "red;green;red;blue;green;yellow"
task("h20", "parse", "Read in/tags.txt (one line of ';'-separated tags). Write out/result.json: the JSON array of distinct tags in first-seen order.",
     {"in/tags.txt": tags}, "out/result.json", "json", list(OrderedDict.fromkeys(tags.split(";"))))

# ---- validation ----
users = [{"id": 1, "email": "a@x"}, {"id": 2}, {"id": 3, "email": "c@x"}, {"id": 4, "name": "d"}]
task("h21", "validate", "Read in/users.json (array of objects with integer id). Write out/result.json: the JSON array of ids of objects that have no \"email\" key, in input order.",
     {"in/users.json": json.dumps(users)}, "out/result.json", "json", [u["id"] for u in users if "email" not in u])
people = [{"id": "p1", "age": 34}, {"id": "p2", "age": -3}, {"id": "p3", "age": 151}, {"id": "p4", "age": 0}, {"id": "p5", "age": 150}]
task("h22", "validate", "Read in/people.json (array of {id, age} with integer ages). Write out/result.txt with one line per record \"<id> valid\" if 0 <= age <= 150 else \"<id> invalid\".",
     {"in/people.json": json.dumps(people)}, "out/result.txt", "text", "\n".join(f"{p['id']} {'valid' if 0 <= p['age'] <= 150 else 'invalid'}" for p in people))
ids = ["a7", "b2", "c9", "b2", "a7"]
def firstdup(xs):
    seen = set()
    for x in xs:
        if x in seen: return "duplicate:" + x
        seen.add(x)
    return "ok"
task("h23", "validate", "Read in/ids.json (array of strings). Write out/result.txt: \"ok\" if all are distinct, else \"duplicate:<id>\" for the first id that repeats an earlier one.",
     {"in/ids.json": json.dumps(ids)}, "out/result.txt", "text", firstdup(ids))
schema = ["host", "port", "user", "timeout"]
config = {"host": "h", "user": "u", "debug": True}
task("h24", "validate", "Read in/schema.json (array of required key names) and in/config.json (object). Write out/result.json: the array of required keys missing from config, in schema order.",
     {"in/schema.json": json.dumps(schema), "in/config.json": json.dumps(config)}, "out/result.json", "json", [k for k in schema if k not in config])
seq = "1\n3\n3\n8\n5\n9"
ss = [int(x) for x in lines(seq)]
task("h25", "validate", "Read in/seq.txt (one integer per line). Write out/result.txt: \"sorted\" if the sequence is non-decreasing, otherwise \"unsorted at <n>\" where n is the 1-based line of the first value smaller than its predecessor.",
     {"in/seq.txt": seq}, "out/result.txt", "text", next((f"unsorted at {i+1}" for i in range(1, len(ss)) if ss[i] < ss[i-1]), "sorted"))

# ---- JSON reshaping ----
items = [{"id": "u1", "name": "Ada"}, {"id": "u2", "name": "Bob"}, {"id": "u3", "name": "Cy"}]
task("h26", "reshape", "Read in/items.json (array of {id, name}). Write out/result.json: an object mapping each id to its name.",
     {"in/items.json": json.dumps(items)}, "out/result.json", "json", {i["id"]: i["name"] for i in items})
prof = {"user": {"name": "Ada", "tags": ["x", "y", "z"], "meta": {"active": True}}}
task("h27", "reshape", "Read in/profile.json ({user: {name, tags, ...}}). Write out/result.json: {\"name\": user.name, \"tag_count\": number of user.tags}.",
     {"in/profile.json": json.dumps(prof)}, "out/result.json", "json", {"name": "Ada", "tag_count": 3})
members = [{"team": "red", "member": "a"}, {"team": "blue", "member": "b"}, {"team": "red", "member": "c"}, {"team": "green", "member": "d"}, {"team": "blue", "member": "e"}]
grp = {}
for m in members: grp.setdefault(m["team"], []).append(m["member"])
task("h28", "reshape", "Read in/members.json (array of {team, member}). Write out/result.json: an object mapping each team to the array of its members in input order.",
     {"in/members.json": json.dumps(members)}, "out/result.json", "json", grp)
recs = [{"id": 1, "status": "open", "owner": "a", "prio": 2}, {"id": 2, "status": "done", "owner": "b", "prio": 1}]
task("h29", "reshape", "Read in/records.json (array of objects). Write out/result.json: the same array keeping only the \"id\" and \"status\" fields of each object.",
     {"in/records.json": json.dumps(recs)}, "out/result.json", "json", [{"id": r["id"], "status": r["status"]} for r in recs])
codes = {"US": "United States", "FR": "France", "JP": "Japan"}
task("h30", "reshape", "Read in/codes.json (object of string to string). Write out/result.json: the inverted object mapping each value to its key.",
     {"in/codes.json": json.dumps(codes)}, "out/result.json", "json", {v: k for k, v in codes.items()})

# ---- file walking ----
tree = {"in/docs/a.txt": "one", "in/docs/sub/b.txt": "two", "in/docs/sub/c.md": "three", "in/docs/sub/deep/d.txt": "four", "in/docs/e.log": "five"}
task("h31", "walk", "Recursively walk in/docs. Write out/result.txt containing the integer number of regular files under it.",
     tree, "out/result.txt", "text", str(len(tree)))
txts = sorted(p for p in tree if p.endswith(".txt"))
task("h32", "walk", "Recursively walk in/docs. Write out/result.txt with the paths (relative to in/docs, '/'-separated) of all files ending in .txt, sorted lexicographically, one per line.",
     tree, "out/result.txt", "text", "\n".join(p[len("in/docs/"):] for p in txts))

Path(__file__).with_name("manifest.json").write_text(json.dumps({"version": "heldout-v0", "tasks": T}, indent=2) + "\n")
print(len(T), "tasks")
