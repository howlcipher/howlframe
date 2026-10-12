"""Held-out-v3 task specs (frozen before any records/JSON slice or reference solution).

Author: Assurance seat bot-assurance-0001 (not the implementer). Base: main d887b96b.
Freeze date: 2026-10-12.
Distinct from token-efficiency-v0, falsifiable-v0 (harness), token-efficiency-heldout-v0,
token-efficiency-heldout-v1 and token-efficiency-heldout-v2. Each task lists input files,
one output path, an output kind and the expected output. Expected outputs come from this
spec-time oracle; it is part of the freeze and is never counted or verified as a
reference.

Output comparison (verifier contract, same as heldout-v0, heldout-v1 and heldout-v2):
  text: exact match after removing trailing newlines from both sides.
  json: decoded-value equality.
"""
import json
from pathlib import Path

T = []
def task(id, cat, desc, inputs, out, kind, expect):
    T.append({"id": id, "category": cat, "description": desc, "inputs": inputs,
              "output": out, "kind": kind, "expected": expect})

def L(s): return s.split("\n")
def J(v): return json.dumps(v)

# ---- records (x01-x17) ----
cfg = {"db": {"primary": {"host": "db1.internal", "port": 5432}, "replicas": 2},
       "cache": {"ttl": 30, "enabled": False}, "owner": None}
paths = "db.primary.host\ndb.primary.port\ncache.enabled\ndb.secondary.host\nowner\ncache.ttl.unit"
def get_in(o, p):
    for k in p.split("."):
        if not isinstance(o, dict) or k not in o: return "missing"
        o = o[k]
    return json.dumps(o)
task("x01", "records", "Read in/config.json (nested JSON objects) and in/paths.txt (one dotted key path per line, e.g. a.b.c). For each path, follow the keys from the top-level object. If every key exists, write the value found encoded as compact JSON (strings quoted, null as null, booleans as true/false); if any key is absent or an intermediate value is not an object, write missing. One result per line in out/result.txt.",
     {"in/config.json": J(cfg), "in/paths.txt": paths}, "out/result.txt", "text", "\n".join(get_in(cfg, p) for p in L(paths)))

base = {"name": "svc", "limits": {"cpu": 2, "mem": 512, "io": {"read": 10, "write": 5}}, "tags": ["a", "b"], "debug": False}
over = {"limits": {"mem": 1024, "io": {"write": 8}}, "tags": ["c"], "region": "eu"}
def deep_merge(a, b):
    r = dict(a)
    for k, v in b.items():
        r[k] = deep_merge(r[k], v) if isinstance(v, dict) and isinstance(r.get(k), dict) else v
    return r
task("x02", "records", "Read in/base.json and in/override.json (JSON objects). Deep-merge override into base: for each key in override, if both values are objects merge them recursively, otherwise the override value replaces the base value (arrays are replaced, not concatenated). Keys only in base are kept. Write the merged object to out/result.json.",
     {"in/base.json": J(base), "in/override.json": J(over)}, "out/result.json", "json", deep_merge(base, over))

nest = {"server": {"http": {"port": 80, "tls": True}, "name": "edge"}, "retries": 3, "paths": {"logs": "/var/log", "empty": {}}}
def flatten(o, pre=""):
    r = {}
    for k, v in o.items():
        key = pre + k
        if isinstance(v, dict) and v: r.update(flatten(v, key + "."))
        else: r[key] = v
    return r
task("x03", "records", "Read in/nested.json (a JSON object whose values may be nested objects). Write out/result.json: a flat object whose keys are the dot-joined key paths to every non-object value (an empty object counts as a value and is kept as {}), with the values unchanged.",
     {"in/nested.json": J(nest)}, "out/result.json", "json", flatten(nest))

flat = {"app.name": "howl", "app.version.major": 3, "app.version.minor": 1, "log.level": "warn", "workers": 4}
def unflatten(f):
    r = {}
    for k, v in f.items():
        cur = r; ps = k.split(".")
        for p in ps[:-1]: cur = cur.setdefault(p, {})
        cur[ps[-1]] = v
    return r
task("x04", "records", "Read in/flat.json (an object whose keys are dot-separated paths; no path is a prefix of another key). Write out/result.json: the equivalent nested object, creating an object for each path segment.",
     {"in/flat.json": J(flat)}, "out/result.json", "json", unflatten(flat))

contacts = [{"id": 1, "email": "a@x.io"}, {"id": 2, "email": None}, {"id": 3}, {"id": 4, "email": "d@x.io"}, {"id": 5, "email": None}, {"id": 6}]
cls = {"present": [], "null": [], "absent": []}
for c in contacts:
    if "email" not in c: cls["absent"].append(c["id"])
    elif c["email"] is None: cls["null"].append(c["id"])
    else: cls["present"].append(c["id"])
task("x05", "records", "Read in/contacts.json (array of objects with an integer id and an optional email key that may be a string or null). Write out/result.json: {\"present\": [...], \"null\": [...], \"absent\": [...]}, the ids whose email is a string, is null, or is not present at all, each in input order.",
     {"in/contacts.json": J(contacts)}, "out/result.json", "json", cls)

sales = [{"region": "north", "product": "kite", "units": 4}, {"region": "south", "product": "yoyo", "units": 7},
         {"region": "north", "product": "yoyo", "units": 2}, {"region": "north", "product": "kite", "units": 1},
         {"region": "south", "product": "ball", "units": 3}]
g = {}
for s in sales:
    e = g.setdefault(s["region"], {"units": 0, "products": set()})
    e["units"] += s["units"]; e["products"].add(s["product"])
g = {k: {"units": v["units"], "products": sorted(v["products"])} for k, v in g.items()}
task("x06", "records", "Read in/sales.json (array of {region, product, units}). Write out/result.json: an object mapping each region to a record {\"units\": total integer units, \"products\": array of the distinct product names in that region sorted ascending}.",
     {"in/sales.json": J(sales)}, "out/result.json", "json", g)

rows = [{"fname": "Ida", "lname": "Moss", "dob": "1990-02-01"}, {"fname": "Raj", "lname": "Kaur", "zip": "48201"}]
mapping = {"fname": "first_name", "lname": "last_name", "dob": "birth_date"}
task("x07", "records", "Read in/rows.json (array of objects) and in/rename.json (object of old key to new key). Write out/result.json: the array with every key that appears in rename.json renamed to its new name; other keys and all values unchanged.",
     {"in/rows.json": J(rows), "in/rename.json": J(mapping)}, "out/result.json", "json", [{mapping.get(k, k): v for k, v in r.items()} for r in rows])

doc = {"zeta": 1, "alpha": {"m": [3, {"y": True, "b": None}], "c": "q"}, "Beta": "x y"}
task("x08", "records", "Read in/doc.json (any JSON object). Write out/result.txt with one line: the same value as canonical compact JSON, with the keys of every object (at any depth) sorted in ascending byte order, no whitespace between tokens (separators \",\" and \":\"), array order unchanged. The output is compared as text.",
     {"in/doc.json": J(doc)}, "out/result.txt", "text", json.dumps(doc, sort_keys=True, separators=(",", ":")))

old = {"host": "a", "port": 80, "debug": True, "mode": "x"}
new = {"host": "a", "port": 8080, "mode": "x", "timeout": 5}
diff = {"added": sorted(k for k in new if k not in old), "removed": sorted(k for k in old if k not in new),
        "changed": sorted(k for k in new if k in old and new[k] != old[k])}
task("x09", "records", "Read in/old.json and in/new.json (flat JSON objects). Write out/result.json: {\"added\": keys only in new, \"removed\": keys only in old, \"changed\": keys in both whose values differ}, each an array sorted ascending.",
     {"in/old.json": J(old), "in/new.json": J(new)}, "out/result.json", "json", diff)

tickets = [{"id": "T1", "meta": {"tags": ["ui", "urgent"]}}, {"id": "T2", "meta": {"tags": []}},
           {"id": "T3", "meta": {"tags": ["urgent"]}}, {"id": "T4", "meta": {}}, {"id": "T5", "meta": {"tags": ["backend"]}}]
task("x10", "records", "Read in/tickets.json (array of {id, meta}; meta is an object that may have a tags array). Write out/result.txt with the id of every ticket whose meta.tags contains \"urgent\", one per line in input order.",
     {"in/tickets.json": J(tickets)}, "out/result.txt", "text", "\n".join(t["id"] for t in tickets if "urgent" in t["meta"].get("tags", [])))

accts = [{"login": "kim", "user": {"profile": {"city": "Oslo"}}}, {"login": "lu", "user": {"profile": {}}},
         {"login": "mo", "user": {}}, {"login": "ny", "user": {"profile": {"city": None}}}, {"login": "ot", "user": {"profile": {"city": "Lima"}}}]
def city(a):
    c = a.get("user", {}).get("profile", {}).get("city")
    return c if c is not None else "unknown"
task("x11", "records", "Read in/accounts.json (array of {login, user}). Write out/result.txt with one line per account: \"<login>: <city>\" where city is user.profile.city, or unknown if user.profile or city is missing or city is null.",
     {"in/accounts.json": J(accts)}, "out/result.txt", "text", "\n".join(f"{a['login']}: {city(a)}" for a in accts))

people = [{"first": "Ada", "last": "King", "age": 36}, {"first": "Alan", "last": "Turing", "age": 41}, {"first": "Grace", "last": "Hopper", "age": 85}]
task("x12", "records", "Read in/people.json (array of {first, last, age}). Write out/result.json: an object keyed by \"<last>, <first>\" mapping to the integer age.",
     {"in/people.json": J(people)}, "out/result.json", "json", {f"{p['last']}, {p['first']}": p["age"] for p in people})

jl = [{"level": "info", "ms": 12}, {"level": "warn", "ms": 40}, {"level": "info", "ms": 31}, {"level": "error", "ms": 7}, {"level": "warn", "ms": 22}]
mx = {}
for e in jl: mx[e["level"]] = max(mx.get(e["level"], e["ms"]), e["ms"])
task("x13", "records", "Read in/requests.jsonl (one JSON object {level, ms} per line). Write out/result.json: an object mapping each level to the largest integer ms seen for it.",
     {"in/requests.jsonl": "\n".join(J(e) for e in jl)}, "out/result.json", "json", mx)

cents = {"tea": 450, "cake": 1205, "water": 99, "gift": 10000}
task("x14", "records", "Read in/prices.json (object of item to integer price in cents). Write out/result.json: an object with the same keys, each value the price as a string \"$D.CC\" (dollars without leading zeros, at least \"0\"; exactly two cent digits).",
     {"in/prices.json": J(cents)}, "out/result.json", "json", {k: f"${v // 100}.{v % 100:02d}" for k, v in cents.items()})

tree = {"a": [1, {"b": {"c": []}}], "d": {"e": 2}}
def depth(v):
    if isinstance(v, dict): return 1 + max((depth(x) for x in v.values()), default=0)
    if isinstance(v, list): return 1 + max((depth(x) for x in v), default=0)
    return 0
task("x15", "records", "Read in/tree.json (any JSON value). Write out/result.txt with its nesting depth as an integer: a scalar (number, string, boolean, null) has depth 0; an object or array has depth 1 plus the largest depth among its values or elements (1 if empty).",
     {"in/tree.json": J(tree)}, "out/result.txt", "text", str(depth(tree)))

env = {"PATH": "/bin", "HOME": "/root", "LANG": "C", "EDITOR": "vi"}
task("x16", "records", "Read in/env.json (object of string to string). Write out/result.json: an array of {\"key\": k, \"value\": v} records, one per entry, sorted by key ascending.",
     {"in/env.json": J(env)}, "out/result.json", "json", [{"key": k, "value": env[k]} for k in sorted(env)])

target = {"svc": {"replicas": 1, "image": "v1"}, "owner": "ops"}
ops = [{"op": "set", "path": "svc.replicas", "value": 3}, {"op": "set", "path": "svc.env.MODE", "value": "prod"},
       {"op": "del", "path": "owner"}, {"op": "del", "path": "svc.missing"}, {"op": "set", "path": "svc.image", "value": "v2"}]
import copy
def patch(o, ops):
    o = copy.deepcopy(o)
    for p in ops:
        ks = p["path"].split("."); cur = o
        if p["op"] == "set":
            for k in ks[:-1]: cur = cur.setdefault(k, {})
            cur[ks[-1]] = p["value"]
        else:
            for k in ks[:-1]:
                cur = cur.get(k, {})
            cur.pop(ks[-1], None)
    return o
task("x17", "records", "Read in/doc.json (object) and in/ops.json (array of {op, path, value?}; op is \"set\" or \"del\", path is dot-separated). Apply the ops in order: set assigns value at path, creating missing intermediate objects; del removes the key at path and does nothing if it is absent. Write the resulting object to out/result.json.",
     {"in/doc.json": J(target), "in/ops.json": J(ops)}, "out/result.json", "json", patch(target, ops))

# ---- text (x18-x20) ----
cont = "run build \\\n  --fast \\\n  --quiet\necho done\nlint \\\n  src"
def join_cont(s):
    out, cur = [], ""
    for l in L(s):
        if l.endswith("\\"): cur += l[:-1]
        else: out.append(cur + l); cur = ""
    return "\n".join(out)
task("x18", "text", "Read in/script.txt. A line ending in a backslash continues on the next line: remove the trailing backslash and append the next line to it unchanged (keep its leading spaces). Repeat for chains. Write the joined logical lines to out/result.txt.",
     {"in/script.txt": cont}, "out/result.txt", "text", join_cont(cont))
ini = "[core]\nname=alpha\n[net]\nhost=10.0.0.1\nport=9000\n\n[extra]\nnet=no"
def section(s, want):
    out, cur = [], None
    for l in L(s):
        if l.startswith("[") and l.endswith("]"): cur = l[1:-1]
        elif cur == want and l: out.append(l)
    return "\n".join(out)
task("x19", "text", "Read in/app.ini (lines that are either a section header \"[name]\", a key=value line, or empty). Write out/result.txt with the non-empty lines that belong to the section named net (after \"[net]\" and before the next header), in order.",
     {"in/app.ini": ini}, "out/result.txt", "text", section(ini, "net"))
paras = "one two three\nfour\n\nfive six\n\n\nseven eight nine ten"
def para_counts(s):
    ps, cur = [], 0
    for l in L(s) + [""]:
        if l.strip() == "":
            if cur: ps.append(cur); cur = 0
        else: cur += len(l.split())
    return "\n".join(f"{i + 1}: {c}" for i, c in enumerate(ps))
task("x20", "text", "Read in/doc.txt. Paragraphs are runs of non-blank lines separated by one or more blank lines. Write out/result.txt with one line per paragraph, \"<n>: <words>\", where n is the 1-based paragraph number and words is the number of space-separated words in it.",
     {"in/doc.txt": paras}, "out/result.txt", "text", para_counts(paras))

# ---- table (x21-x23) ----
grid = "metric,mon,tue,wed\ncpu,40,55,61\nmem,70,72,68"
gr = [r.split(",") for r in L(grid)]
task("x21", "table", "Read in/grid.csv (comma-separated, no quoting, every row the same width). Write out/result.csv: the table transposed (row i column j becomes row j column i), comma-separated.",
     {"in/grid.csv": grid}, "out/result.csv", "text", "\n".join(",".join(r[j] for r in gr) for j in range(len(gr[0]))))
sparse = "site,owner,status\nalpha,ann,up\nbeta,,down\ngamma,,\ndelta,dan,\nepsilon,,up"
def ffill(s):
    rs = [r.split(",") for r in L(s)]
    out, prev = [rs[0]], None
    for r in rs[1:]:
        if prev: r = [v if v != "" else prev[i] for i, v in enumerate(r)]
        out.append(r); prev = r
    return "\n".join(",".join(r) for r in out)
task("x22", "table", "Read in/sites.csv (header row, comma-separated, no quoting; some cells are empty). Fill every empty cell with the value in the same column of the row above it after that row was filled (the first data row has no empty cells). Write the filled table, header included, to out/result.csv.",
     {"in/sites.csv": sparse}, "out/result.csv", "text", ffill(sparse))
emp = "id,name\n7,ivy\n3,jon\n9,kai"
dept = "id,dept\n3,ops\n7,eng\n5,hr"
dm = {r.split(",")[0]: r.split(",")[1] for r in L(dept)[1:]}
task("x23", "table", "Read in/emp.csv (header id,name) and in/dept.csv (header id,dept). Left-join on id: write out/result.csv with header id,name,dept and one row per emp.csv row in its order; dept is empty when the id has no match.",
     {"in/emp.csv": emp, "in/dept.csv": dept}, "out/result.csv", "text", "\n".join(["id,name,dept"] + [f"{r},{dm.get(r.split(',')[0], '')}" for r in L(emp)[1:]]))

# ---- numbers (x24-x26) ----
import math
gl = "12,18,30"
gv = [int(x) for x in gl.split(",")]
lcm = 1
for v in gv: lcm = lcm * v // math.gcd(lcm, v)
task("x24", "numbers", "Read in/vals.txt (one line of comma-separated positive integers). Write out/result.json: {\"gcd\": greatest common divisor, \"lcm\": least common multiple} of all of them, as integers.",
     {"in/vals.txt": gl}, "out/result.json", "json", {"gcd": math.gcd(*gv), "lcm": lcm})
dr = "38\n0\n9\n123456789\n1000"
def droot(n):
    while n >= 10: n = sum(int(c) for c in str(n))
    return n
task("x25", "numbers", "Read in/nums.txt (one non-negative integer per line). For each, repeatedly replace it with the sum of its decimal digits until it is a single digit. Write \"<n> -> <digit>\" per line to out/result.txt.",
     {"in/nums.txt": dr}, "out/result.txt", "text", "\n".join(f"{x} -> {droot(int(x))}" for x in L(dr)))
votes = "red 13\nblue 29\ngreen 8"
vr = [l.split(" ") for l in L(votes)]
tot = sum(int(v) for _, v in vr)
task("x26", "numbers", "Read in/votes.txt (lines \"<name> <count>\" with non-negative integer counts, positive total). Write out/result.txt with one line per input line: \"<name> <p>%\" where p = count * 100 / total rounded down to an integer.",
     {"in/votes.txt": votes}, "out/result.txt", "text", "\n".join(f"{n} {int(c) * 100 // tot}%" for n, c in vr))

# ---- lists (x27-x29) ----
deep = [1, [2, [3, [4]], 5], [], [[6]]]
def flat_list(v): return [y for x in v for y in (flat_list(x) if isinstance(x, list) else [x])]
task("x27", "lists", "Read in/deep.json (an array whose elements are integers or arrays nested to any depth). Write out/result.json: one flat array of all the integers in left-to-right order.",
     {"in/deep.json": J(deep)}, "out/result.json", "json", flat_list(deep))
la, lb = ["git", "make", "curl", "jq", "make"], ["jq", "tar", "git", "zip"]
task("x28", "lists", "Read in/a.json and in/b.json (arrays of strings). Write out/result.json: {\"only_a\": elements of a not in b, \"only_b\": elements of b not in a}, each in its own input order with duplicates removed (keep first occurrence).",
     {"in/a.json": J(la), "in/b.json": J(lb)}, "out/result.json", "json",
     {"only_a": list(dict.fromkeys(x for x in la if x not in lb)), "only_b": list(dict.fromkeys(x for x in lb if x not in la))})
hist = ["home", "docs", "home", "blog", "docs", "about"]
def keep_last(v):
    seen, out = set(), []
    for x in reversed(v):
        if x not in seen: seen.add(x); out.append(x)
    return out[::-1]
task("x29", "lists", "Read in/visits.json (array of strings). Write out/result.json: the array with duplicates removed keeping only the LAST occurrence of each string, in the order those last occurrences appear.",
     {"in/visits.json": J(hist)}, "out/result.json", "json", keep_last(hist))

# ---- strings (x30-x32) ----
pairs = "listen silent\nevil vile\napple paple\nrat car\nab abb"
task("x30", "strings", "Read in/pairs.txt (lines of two lower-case words separated by one space). Write out/result.txt with yes if the two words are anagrams (same letters with the same counts), else no, one per line.",
     {"in/pairs.txt": pairs}, "out/result.txt", "text", "\n".join("yes" if sorted(a) == sorted(b) else "no" for a, b in (l.split(" ") for l in L(pairs))))
rng = "1-3,7,10-12,15"
def expand(s):
    out = []
    for p in s.split(","):
        if "-" in p: a, b = p.split("-"); out += [str(i) for i in range(int(a), int(b) + 1)]
        else: out.append(p)
    return ",".join(out)
task("x31", "strings", "Read in/ranges.txt (one line of comma-separated items, each a non-negative integer or an inclusive range \"a-b\" with a <= b). Write out/result.txt with every number listed, comma-separated, in order.",
     {"in/ranges.txt": rng}, "out/result.txt", "text", expand(rng))
pre = "interstellar\ninternet\ninterval\ninternal"
import os
task("x32", "strings", "Read in/words.txt (one word per line). Write out/result.txt with the longest common prefix of all the lines (empty if none).",
     {"in/words.txt": pre}, "out/result.txt", "text", os.path.commonprefix(L(pre)))

# ---- logic (x33-x35) ----
iv = "5-8\n1-3\n2-4\n10-11\n8-9"
def merge(s):
    xs = sorted([int(a), int(b)] for a, b in (l.split("-") for l in L(s)))
    out = []
    for a, b in xs:
        if out and a <= out[-1][1]: out[-1][1] = max(out[-1][1], b)
        else: out.append([a, b])
    return "\n".join(f"{a}-{b}" for a, b in out)
task("x33", "logic", "Read in/spans.txt (one inclusive integer interval \"a-b\" per line, a <= b, unsorted). Merge intervals that overlap or touch at an endpoint (e.g. 5-8 and 8-9). Write the merged intervals sorted by start, one \"a-b\" per line, to out/result.txt.",
     {"in/spans.txt": iv}, "out/result.txt", "text", merge(iv))
doy = "1\n59\n60\n200\n365"
ML = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]
def md(n):
    m = 0
    while n > ML[m]: n -= ML[m]; m += 1
    return f"{m + 1:02d}-{n:02d}"
task("x34", "logic", "Read in/days.txt (one day-of-year integer 1..365 per line, non-leap year). Write out/result.txt with each as MM-DD (two digits each), one per line.",
     {"in/days.txt": doy}, "out/result.txt", "text", "\n".join(md(int(x)) for x in L(doy)))
deps = {"deploy": ["test", "build"], "test": ["build"], "build": ["fetch"], "fetch": [], "lint": ["fetch"], "docs": []}
def topo(d):
    done, out = set(), []
    while len(out) < len(d):
        ready = sorted(k for k in d if k not in done and all(x in done for x in d[k]))
        k = ready[0]; done.add(k); out.append(k)
    return out
task("x35", "logic", "Read in/deps.json (object of task name to array of task names it depends on; no cycles). Write out/result.json: an array of all task names in an order where every task comes after its dependencies; at each step pick the alphabetically smallest task whose dependencies are all already placed.",
     {"in/deps.json": J(deps)}, "out/result.json", "json", topo(deps))

Path(__file__).with_name("manifest.json").write_text(json.dumps({"version": "heldout-v3", "tasks": T}, indent=2) + "\n")
print(len(T), "tasks")
