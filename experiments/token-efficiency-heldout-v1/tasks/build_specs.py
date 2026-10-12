"""Held-out-v1 task specs (frozen before any library code or reference solution).

Distinct from token-efficiency-v0 and token-efficiency-heldout-v0. Each task
lists input files, one output path, an output kind and the expected output.
Expected outputs come from this spec-time oracle; it is part of the freeze and
is never counted or verified as a reference.

Output comparison (verifier contract, same as heldout-v0):
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

# ---- text (v01-v05) ----
poem = "Roses are red\nviolets are blue\nsugar is sweet\nAND so are you\nthe end"
task("v01", "text", "Read in/poem.txt. Write out/result.txt with the first word of each line, one per line, in order.",
     {"in/poem.txt": poem}, "out/result.txt", "text", "\n".join(l.split(" ")[0] for l in L(poem)))
task("v02", "text", "Read in/poem.txt. Write out/result.txt where every line has its words in reverse order (words are separated by single spaces).",
     {"in/poem.txt": poem}, "out/result.txt", "text", "\n".join(" ".join(reversed(l.split(" "))) for l in L(poem)))
task("v03", "text", "Read in/poem.txt. Write out/result.txt containing only the first 3 characters of every line, one per line.",
     {"in/poem.txt": poem}, "out/result.txt", "text", "\n".join(l[:3] for l in L(poem)))
task("v04", "text", "Read in/poem.txt. Write out/result.txt with the lines sorted in ascending byte order (plain string comparison; uppercase sorts before lowercase).",
     {"in/poem.txt": poem}, "out/result.txt", "text", "\n".join(sorted(L(poem))))
task("v05", "text", "Read in/poem.txt. Write out/result.txt with only the lines whose word count is exactly 3, in input order.",
     {"in/poem.txt": poem}, "out/result.txt", "text", "\n".join(l for l in L(poem) if len(l.split(" ")) == 3))

# ---- tabular (v06-v10) ----
tsv = "sku\tqty\tprice\nA1\t3\t250\nB7\t10\t99\nC2\t1\t1200\nA9\t0\t45\nD4\t7\t300"
hdr, *rws = L(tsv)
rr = [r.split("\t") for r in rws]
task("v06", "table", "Read in/stock.tsv (tab-separated, header row sku,qty,price). Write out/result.txt with the sku of every row whose integer qty is 0.",
     {"in/stock.tsv": tsv}, "out/result.txt", "text", "\n".join(r[0] for r in rr if int(r[1]) == 0))
task("v07", "table", "Read in/stock.tsv. Write out/result.txt containing the integer total value: the sum over rows of qty * price.",
     {"in/stock.tsv": tsv}, "out/result.txt", "text", str(sum(int(r[1]) * int(r[2]) for r in rr)))
task("v08", "table", "Read in/stock.tsv. Write out/result.txt with the sku of the row with the highest integer price.",
     {"in/stock.tsv": tsv}, "out/result.txt", "text", max(rr, key=lambda r: int(r[2]))[0])
task("v09", "table", "Read in/stock.tsv. Write out/result.txt with the data rows (no header) sorted by sku in ascending string order, each row written back tab-separated.",
     {"in/stock.tsv": tsv}, "out/result.txt", "text", "\n".join("\t".join(r) for r in sorted(rr, key=lambda r: r[0])))
task("v10", "table", "Read in/stock.tsv. Write out/result.json: an object mapping each sku to its integer qty.",
     {"in/stock.tsv": tsv}, "out/result.json", "json", {r[0]: int(r[1]) for r in rr})

# ---- numbers (v11-v15) ----
temps = "18,21,19,25,30,22,17"
ts = [int(x) for x in temps.split(",")]
task("v11", "numbers", "Read in/temps.txt (one line of comma-separated integers). Write out/result.txt with the integer difference between the largest and smallest value.",
     {"in/temps.txt": temps}, "out/result.txt", "text", str(max(ts) - min(ts)))
task("v12", "numbers", "Read in/temps.txt. Write out/result.json: the JSON array of values that are strictly greater than the first value, in input order.",
     {"in/temps.txt": temps}, "out/result.json", "json", [t for t in ts if t > ts[0]])
task("v13", "numbers", "Read in/temps.txt. Write out/result.json: the JSON array of running totals (the i-th element is the sum of the first i+1 values).",
     {"in/temps.txt": temps}, "out/result.json", "json", [sum(ts[:i + 1]) for i in range(len(ts))])
task("v14", "numbers", "Read in/temps.txt. Write out/result.txt with the 1-based position of the largest value (the first one on ties).",
     {"in/temps.txt": temps}, "out/result.txt", "text", str(ts.index(max(ts)) + 1))
task("v15", "numbers", "Read in/temps.txt. Write out/result.json: the values doubled, as a JSON array of integers in input order.",
     {"in/temps.txt": temps}, "out/result.json", "json", [t * 2 for t in ts])

# ---- lists (v16-v20) ----
queue = ["deploy", "test", "lint", "build", "test", "deploy", "audit"]
task("v16", "lists", "Read in/queue.json (array of strings). Write out/result.json: the last 3 elements, in their original order.",
     {"in/queue.json": json.dumps(queue)}, "out/result.json", "json", queue[-3:])
task("v17", "lists", "Read in/queue.json. Write out/result.txt with the 0-based index of the first \"build\", or -1 if absent.",
     {"in/queue.json": json.dumps(queue)}, "out/result.txt", "text", str(queue.index("build") if "build" in queue else -1))
task("v18", "lists", "Read in/queue.json. Write out/result.json: the distinct strings sorted in ascending string order.",
     {"in/queue.json": json.dumps(queue)}, "out/result.json", "json", sorted(set(queue)))
task("v19", "lists", "Read in/queue.json. Write out/result.json: an object mapping each string to how many times it occurs.",
     {"in/queue.json": json.dumps(queue)}, "out/result.json", "json", {q: queue.count(q) for q in queue})
task("v20", "lists", "Read in/queue.json. Write out/result.json: the array without its first and last elements.",
     {"in/queue.json": json.dumps(queue)}, "out/result.json", "json", queue[1:-1])

# ---- strings (v21-v25) ----
task("v21", "strings", "Read in/path.txt (one Unix path). Write out/result.txt with the file extension without the dot (text after the last '.').",
     {"in/path.txt": "/srv/app/releases/v2.3/build.tar.gz"}, "out/result.txt", "text", "gz")
task("v22", "strings", "Read in/word.txt (one word). Write out/result.txt with \"yes\" if it reads the same reversed, else \"no\".",
     {"in/word.txt": "racecar"}, "out/result.txt", "text", "yes")
slug = "  Hello World From  HowlFrame "
task("v23", "strings", "Read in/title.txt. Make a slug: trim, lower-case, split on single spaces, drop empty pieces, join with '-'. Write it to out/result.txt.",
     {"in/title.txt": slug}, "out/result.txt", "text", "-".join(p for p in slug.strip().lower().split(" ") if p))
task("v24", "strings", "Read in/version.txt (MAJOR.MINOR.PATCH). Write out/result.txt with the next minor version (minor + 1, patch reset to 0).",
     {"in/version.txt": "4.17.9"}, "out/result.txt", "text", "4.18.0")
sent = "the quick brown fox jumps over the lazy dog"
task("v25", "strings", "Read in/sentence.txt. Write out/result.txt with the longest word (the first one on ties).",
     {"in/sentence.txt": sent}, "out/result.txt", "text", max(sent.split(" "), key=len))

# ---- records (v26-v30) ----
orders = [{"id": "o1", "cust": "ada", "total": 120}, {"id": "o2", "cust": "bob", "total": 80},
          {"id": "o3", "cust": "ada", "total": 45}, {"id": "o4", "cust": "cy", "total": 200},
          {"id": "o5", "cust": "bob", "total": 15}]
od = json.dumps(orders)
task("v26", "records", "Read in/orders.json (array of {id, cust, total}). Write out/result.json: an object mapping each cust to the integer sum of their totals.",
     {"in/orders.json": od}, "out/result.json", "json", {c: sum(o["total"] for o in orders if o["cust"] == c) for c in {o["cust"] for o in orders}})
task("v27", "records", "Read in/orders.json. Write out/result.txt with the ids of orders whose total is at least 100, one per line in input order.",
     {"in/orders.json": od}, "out/result.txt", "text", "\n".join(o["id"] for o in orders if o["total"] >= 100))
task("v28", "records", "Read in/orders.json. Write out/result.json: the array of distinct cust names in first-seen order.",
     {"in/orders.json": od}, "out/result.json", "json", list(dict.fromkeys(o["cust"] for o in orders)))
task("v29", "records", "Read in/orders.json. Write out/result.txt with the id of the order with the smallest total.",
     {"in/orders.json": od}, "out/result.txt", "text", min(orders, key=lambda o: o["total"])["id"])
task("v30", "records", "Read in/orders.json. Write out/result.json: {\"count\": number of orders, \"total\": integer sum of totals}.",
     {"in/orders.json": od}, "out/result.json", "json", {"count": len(orders), "total": sum(o["total"] for o in orders)})

# ---- logic (v31-v34) ----
task("v31", "logic", "Read in/n.txt (one integer n >= 1). Write out/result.txt with the FizzBuzz lines for 1..n: \"FizzBuzz\" for multiples of 15, \"Fizz\" for 3, \"Buzz\" for 5, else the number.",
     {"in/n.txt": "15"}, "out/result.txt", "text", "\n".join("FizzBuzz" if i % 15 == 0 else "Fizz" if i % 3 == 0 else "Buzz" if i % 5 == 0 else str(i) for i in range(1, 16)))
br = "(a[b]{c})(]"
def balanced(s):
    st, pairs = [], {")": "(", "]": "[", "}": "{"}
    for i, ch in enumerate(s):
        if ch in "([{": st.append(ch)
        elif ch in pairs:
            if not st or st.pop() != pairs[ch]: return f"error at {i}"
    return "ok" if not st else "unclosed"
task("v32", "logic", "Read in/brackets.txt (one line). Check (), [] and {} nesting. Write \"ok\" if balanced, \"unclosed\" if brackets remain open at the end, or \"error at <i>\" (0-based character index) at the first closing bracket that does not match.",
     {"in/brackets.txt": br}, "out/result.txt", "text", balanced(br))
grid = "#..#\n....\n##.#"
task("v33", "logic", "Read in/grid.txt (rows of '#' and '.'). Write out/result.json: an array with the integer count of '#' in each row.",
     {"in/grid.txt": grid}, "out/result.json", "json", [r.count("#") for r in L(grid)])
pairs = "b>c\na>b\nc>d"
task("v34", "logic", "Read in/edges.txt (lines \"x>y\" forming one chain). Write out/result.txt with the chain from its start to its end joined by '>' (here the start is the only node that is never on the right).",
     {"in/edges.txt": pairs}, "out/result.txt", "text", "a>b>c>d")

Path(__file__).with_name("manifest.json").write_text(json.dumps({"version": "heldout-v1", "tasks": T}, indent=2) + "\n")
print(len(T), "tasks")
