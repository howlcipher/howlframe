"""Held-out-v2 task specs (frozen before any control-flow/logic built-in or reference solution).

Author: R&D seat bot-rnd-0001 (not the implementer). Base: main be5fc0e2.
Distinct from token-efficiency-v0, token-efficiency-heldout-v0 and
token-efficiency-heldout-v1. Each task lists input files, one output path, an
output kind and the expected output. Expected outputs come from this spec-time
oracle; it is part of the freeze and is never counted or verified as a
reference.

Output comparison (verifier contract, same as heldout-v0 and heldout-v1):
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

# ---- text (w01-w04) ----
para = "the quick agent reads every spec twice before it writes a single line of code"
def wrap(s, width):
    lines, cur = [], ""
    for w in s.split(" "):
        if cur == "": cur = w
        elif len(cur) + 1 + len(w) <= width: cur += " " + w
        else: lines.append(cur); cur = w
    lines.append(cur)
    return "\n".join(lines)
task("w01", "text", "Read in/para.txt (one line of words separated by single spaces). Greedy word-wrap it to width 20: put as many words on a line as fit (joined by single spaces, line length <= 20), then start a new line. Write the wrapped lines to out/result.txt.",
     {"in/para.txt": para}, "out/result.txt", "text", wrap(para, 20))
log = "start\nstart\nping\nping\nping\nstop\nping\nstop\nstop"
def squeeze(ls):
    out = []
    for l in ls:
        if not out or out[-1] != l: out.append(l)
    return out
task("w02", "text", "Read in/events.txt. Collapse runs of consecutive identical lines into one line (like uniq) and write the result to out/result.txt.",
     {"in/events.txt": log}, "out/result.txt", "text", "\n".join(squeeze(L(log))))
src = "# settings\nhost = example.org  # primary\n\nport = 8080\n   # indented comment\nmode = fast#inline"
def strip_comments(s):
    out = []
    for l in L(s):
        i = l.find("#")
        if i >= 0: l = l[:i]
        l = l.rstrip(" ")
        if l.strip(" ") != "": out.append(l)
    return "\n".join(out)
task("w03", "text", "Read in/settings.txt. On every line remove everything from the first '#' to the end of the line, then remove trailing spaces. Drop lines that are empty or only spaces after that. Write the remaining lines to out/result.txt in order.",
     {"in/settings.txt": src}, "out/result.txt", "text", strip_comments(src))
heads = "release notes for march\nknown issues\nupgrade guide and faq"
task("w04", "text", "Read in/headings.txt (lower-case words separated by single spaces). Write out/result.txt with every line in title case: the first letter of each word upper-cased, the rest unchanged.",
     {"in/headings.txt": heads}, "out/result.txt", "text", "\n".join(" ".join(w[:1].upper() + w[1:] for w in l.split(" ")) for l in L(heads)))

# ---- table (w05-w08) ----
tx = "date,account,amount\n2026-01-03,cash,500\n2026-01-09,card,-120\n2026-02-01,cash,-75\n2026-02-14,card,300\n2026-02-20,savings,1000\n2026-03-02,cash,-40"
txr = [r.split(",") for r in L(tx)[1:]]
bal = {}
for r in txr: bal[r[1]] = bal.get(r[1], 0) + int(r[2])
task("w05", "table", "Read in/ledger.csv (header date,account,amount; comma-separated, no quoting; amount is a signed integer). Write out/result.json: an object mapping each account to its integer balance (sum of its amounts).",
     {"in/ledger.csv": tx}, "out/result.json", "json", bal)
mon = {}
for r in txr: mon[r[0][:7]] = mon.get(r[0][:7], 0) + int(r[2])
task("w06", "table", "Read in/ledger.csv. Write out/result.json: an object mapping each month (the first 7 characters of date, YYYY-MM) to the integer sum of amounts in that month.",
     {"in/ledger.csv": tx}, "out/result.json", "json", mon)
lines = "item,qty,unit\nbolt,40,3\nnut,100,1\nwasher,25,2\ngear,2,150"
lr = [r.split(",") for r in L(lines)[1:]]
task("w07", "table", "Read in/lines.csv (header item,qty,unit; integers for qty and unit). Write out/result.csv: the same table with a fourth column named total equal to qty * unit, header included, rows in input order, comma-separated.",
     {"in/lines.csv": lines}, "out/result.csv", "text", "\n".join(["item,qty,unit,total"] + [",".join(r + [str(int(r[1]) * int(r[2]))]) for r in lr]))
team = "name,role\nAnnika,lead\nBo,dev\nChristopher,dev\nDee,qa"
tr = [r.split(",") for r in L(team)[1:]]
wd = max(len(r[0]) for r in tr)
task("w08", "table", "Read in/team.csv (header name,role). Write out/result.txt with one line per data row: the name padded on the right with spaces to the length of the longest name, then \" | \", then the role.",
     {"in/team.csv": team}, "out/result.txt", "text", "\n".join(r[0].ljust(wd) + " | " + r[1] for r in tr))

# ---- numbers (w09-w12) ----
lat = "120\n87\n340\n95\n101\n77\n410"
lv = [int(x) for x in L(lat)]
task("w09", "numbers", "Read in/latency.txt (one integer per line; the count is odd). Write out/result.txt with the median value.",
     {"in/latency.txt": lat}, "out/result.txt", "text", str(sorted(lv)[len(lv) // 2]))
secs = "59\n3600\n3725\n86399\n0"
def hms(n): return f"{n // 3600}:{n // 60 % 60:02d}:{n % 60:02d}"
task("w10", "numbers", "Read in/seconds.txt (one non-negative integer per line). Write out/result.txt with each value as H:MM:SS (hours not padded, minutes and seconds two digits), one per line.",
     {"in/seconds.txt": secs}, "out/result.txt", "text", "\n".join(hms(int(x)) for x in L(secs)))
ages = "3,17,25,29,31,44,8,19,40,22"
av = [int(x) for x in ages.split(",")]
bk = {}
for a in av:
    k = f"{a // 10 * 10}-{a // 10 * 10 + 9}"
    bk[k] = bk.get(k, 0) + 1
task("w11", "numbers", "Read in/ages.txt (one line of comma-separated non-negative integers). Bucket them by tens. Write out/result.txt with one line per non-empty bucket in ascending order, formatted \"<lo>-<hi>: <count>\" (e.g. \"10-19: 2\").",
     {"in/ages.txt": ages}, "out/result.txt", "text", "\n".join(f"{k}: {bk[k]}" for k in sorted(bk, key=lambda s: int(s.split('-')[0]))))
load = "4,8,6,10,2,9"
ld = [int(x) for x in load.split(",")]
task("w12", "numbers", "Read in/load.txt (one line of comma-separated integers). Write out/result.json: the JSON array of 3-wide moving averages, each rounded down (floor), one per full window, in order.",
     {"in/load.txt": load}, "out/result.json", "json", [sum(ld[i:i + 3]) // 3 for i in range(len(ld) - 2)])

# ---- lists (w13-w16) ----
jobs = ["j1", "j2", "j3", "j4", "j5", "j6", "j7", "j8"]
task("w13", "lists", "Read in/jobs.json (array of strings). Write out/result.json: the elements split into consecutive batches of 3 (the last batch may be shorter), as an array of arrays.",
     {"in/jobs.json": json.dumps(jobs)}, "out/result.json", "json", [jobs[i:i + 3] for i in range(0, len(jobs), 3)])
la, lb = ["a", "b", "c", "d"], ["1", "2"]
def inter(a, b):
    out = []
    for i in range(max(len(a), len(b))):
        if i < len(a): out.append(a[i])
        if i < len(b): out.append(b[i])
    return out
task("w14", "lists", "Read in/left.json and in/right.json (arrays of strings). Write out/result.json: their elements interleaved (left[0], right[0], left[1], right[1], ...); when one runs out, append the rest of the other.",
     {"in/left.json": json.dumps(la), "in/right.json": json.dumps(lb)}, "out/result.json", "json", inter(la, lb))
ring = ["n1", "n2", "n3", "n4", "n5"]
k = 7
task("w15", "lists", "Read in/ring.json (array of strings) and in/k.txt (one non-negative integer k). Write out/result.json: the array rotated left by k positions (k may exceed the length; rotation wraps).",
     {"in/ring.json": json.dumps(ring), "in/k.txt": str(k)}, "out/result.json", "json", ring[k % len(ring):] + ring[:k % len(ring)])
reads = [100, 104, 104, 101, 110]
task("w16", "lists", "Read in/meter.json (array of integers, cumulative meter readings). Write out/result.json: the array of differences between each reading and the one before it (one element shorter than the input).",
     {"in/meter.json": json.dumps(reads)}, "out/result.json", "json", [reads[i] - reads[i - 1] for i in range(1, len(reads))])

# ---- strings (w17-w20) ----
def rle(s):
    out, i = "", 0
    while i < len(s):
        j = i
        while j < len(s) and s[j] == s[i]: j += 1
        out += s[i] + str(j - i); i = j
    return out
task("w17", "strings", "Read in/raw.txt (one line). Run-length encode it: each maximal run of the same character becomes the character followed by the run length (e.g. \"aab\" -> \"a2b1\"). Write it to out/result.txt.",
     {"in/raw.txt": "aaabccddddde"}, "out/result.txt", "text", rle("aaabccddddde"))
idents = "userId\nhttpStatusCode\nname\nretryAfterMs"
def snake(s): return "".join("_" + c.lower() if c.isupper() else c for c in s)
task("w18", "strings", "Read in/idents.txt (one camelCase identifier per line, starting lower-case). Write out/result.txt with each converted to snake_case (insert '_' before every upper-case letter and lower-case it), one per line.",
     {"in/idents.txt": idents}, "out/result.txt", "text", "\n".join(snake(x) for x in L(idents)))
msg = "attack at dawn, zebra!"
def caesar(s, n): return "".join(chr((ord(c) - 97 + n) % 26 + 97) if "a" <= c <= "z" else c for c in s)
task("w19", "strings", "Read in/msg.txt (one line). Shift every lower-case letter a-z forward by 3 places in the alphabet, wrapping z to c; leave every other character unchanged. Write it to out/result.txt.",
     {"in/msg.txt": msg}, "out/result.txt", "text", caesar(msg, 3))
cards = "4111222233334444\n5500000000000004\n340000000000009"
task("w20", "strings", "Read in/cards.txt (one digit string per line). Write out/result.txt with each line masked: every character except the last 4 replaced by '*'.",
     {"in/cards.txt": cards}, "out/result.txt", "text", "\n".join("*" * (len(c) - 4) + c[-4:] for c in L(cards)))

# ---- records (w21-w24) ----
staff = [{"name": "ana", "manager": ""}, {"name": "ben", "manager": "ana"}, {"name": "cai", "manager": "ana"},
         {"name": "dov", "manager": "ben"}, {"name": "eli", "manager": "ana"}, {"name": "fay", "manager": "dov"}]
reps = {}
for s in staff:
    if s["manager"]: reps.setdefault(s["manager"], []).append(s["name"])
task("w21", "records", "Read in/staff.json (array of {name, manager}; manager is \"\" for the top). Write out/result.json: an object mapping each manager who has at least one report to the array of their direct reports' names in input order.",
     {"in/staff.json": json.dumps(staff)}, "out/result.json", "json", reps)
users = [{"id": 1, "name": "ada"}, {"id": 2, "name": "bob"}, {"id": 3, "name": "cy"}]
scores = [{"user": 3, "points": 12}, {"user": 1, "points": 30}, {"user": 3, "points": 5}]
tot = {u["id"]: 0 for u in users}
for s in scores: tot[s["user"]] += s["points"]
task("w22", "records", "Read in/users.json (array of {id, name}) and in/scores.json (array of {user, points}; user is a user id). Write out/result.txt with one line per user in users.json order: \"<name> <total points>\" (0 if the user has no scores).",
     {"in/users.json": json.dumps(users), "in/scores.json": json.dumps(scores)}, "out/result.txt", "text", "\n".join(f"{u['name']} {tot[u['id']]}" for u in users))
stock = {"bolt": 10, "nut": 4, "gear": 0}
moves = [{"sku": "nut", "delta": 6}, {"sku": "bolt", "delta": -3}, {"sku": "cog", "delta": 2}, {"sku": "nut", "delta": -1}]
st2 = dict(stock)
for m in moves: st2[m["sku"]] = st2.get(m["sku"], 0) + m["delta"]
task("w23", "records", "Read in/stock.json (object of sku to integer count) and in/moves.json (array of {sku, delta}). Apply every move in order (a sku not yet in stock starts at 0). Write out/result.json: the final stock object.",
     {"in/stock.json": json.dumps(stock), "in/moves.json": json.dumps(moves)}, "out/result.json", "json", st2)
carts = [{"cart": "c1", "items": [{"sku": "pen", "qty": 2}, {"sku": "ink", "qty": 1}]},
         {"cart": "c2", "items": []},
         {"cart": "c3", "items": [{"sku": "pen", "qty": 5}, {"sku": "pad", "qty": 3}]}]
pq = {}
for c in carts:
    for it in c["items"]: pq[it["sku"]] = pq.get(it["sku"], 0) + it["qty"]
task("w24", "records", "Read in/carts.json (array of {cart, items}; items is an array of {sku, qty}). Write out/result.json: an object mapping each sku to its total integer qty across all carts.",
     {"in/carts.json": json.dumps(carts)}, "out/result.json", "json", pq)

# ---- logic (w25-w37) ----
def primes(n): return [p for p in range(2, n + 1) if all(p % d for d in range(2, p))]
task("w25", "logic", "Read in/n.txt (one integer n >= 2). Write out/result.txt with all primes p <= n separated by single spaces, ascending.",
     {"in/n.txt": "40"}, "out/result.txt", "text", " ".join(map(str, primes(40))))
def collatz(n):
    c = 0
    while n != 1: n = n // 2 if n % 2 == 0 else 3 * n + 1; c += 1
    return c
task("w26", "logic", "Read in/n.txt (one integer n >= 1). Repeatedly replace n by n/2 if it is even or 3n+1 if it is odd, until n is 1. Write out/result.txt with the integer number of steps taken.",
     {"in/n.txt": "27"}, "out/result.txt", "text", str(collatz(27)))
years = "1900\n2000\n2024\n2023\n2100\n2400"
def leap(y): return y % 4 == 0 and (y % 100 != 0 or y % 400 == 0)
task("w27", "logic", "Read in/years.txt (one integer year per line). Write out/result.txt with one line per year: \"<year> leap\" or \"<year> common\" (Gregorian rules: divisible by 4, except centuries unless divisible by 400).",
     {"in/years.txt": years}, "out/result.txt", "text", "\n".join(f"{y} {'leap' if leap(int(y)) else 'common'}" for y in L(years)))
cmds = "open\nclose\nlock\nopen\nunlock"
def door(cs):
    tr = {("closed", "open"): "opened", ("opened", "close"): "closed", ("closed", "lock"): "locked", ("locked", "unlock"): "closed"}
    s = "closed"
    for i, c in enumerate(cs, 1):
        if (s, c) not in tr: return f"error line {i}: cannot {c} when {s}"
        s = tr[(s, c)]
    return s
task("w28", "logic", "Read in/cmds.txt (one command per line). A door starts \"closed\". Allowed transitions: closed+open->opened, opened+close->closed, closed+lock->locked, locked+unlock->closed. Process commands in order; at the first command not allowed in the current state stop and write \"error line <n>: cannot <cmd> when <state>\" (n is 1-based). If all succeed, write the final state. Output goes to out/result.txt.",
     {"in/cmds.txt": cmds}, "out/result.txt", "text", door(L(cmds)))
spend = "30\n45\n10\n60\n5"
def first_over(vals, lim):
    s = 0
    for i, v in enumerate(vals, 1):
        s += v
        if s > lim: return f"over at {i}"
    return "within budget"
task("w29", "logic", "Read in/spend.txt (one integer per line) and in/budget.txt (one integer). Add the values in order; write out/result.txt with \"over at <n>\" for the 1-based line where the running total first exceeds the budget, or \"within budget\" if it never does.",
     {"in/spend.txt": spend, "in/budget.txt": "100"}, "out/result.txt", "text", first_over([int(x) for x in L(spend)], 100))
romans = "XIV\nMCMXCIV\nXL\nMMXXVI\nIX"
def roman(s):
    v = {"I": 1, "V": 5, "X": 10, "L": 50, "C": 100, "D": 500, "M": 1000}
    t = 0
    for i, c in enumerate(s):
        t += -v[c] if i + 1 < len(s) and v[s[i + 1]] > v[c] else v[c]
    return t
task("w30", "logic", "Read in/romans.txt (one valid Roman numeral per line, I V X L C D M with subtractive forms). Write out/result.json: the JSON array of their integer values in order.",
     {"in/romans.txt": romans}, "out/result.json", "json", [roman(r) for r in L(romans)])
nums = "5\n0\n255\n1024"
task("w31", "logic", "Read in/nums.txt (one non-negative integer per line). Write out/result.txt with each number in base 2 (no prefix, \"0\" for zero), one per line.",
     {"in/nums.txt": nums}, "out/result.txt", "text", "\n".join(bin(int(x))[2:] for x in L(nums)))
ids = "79927398713\n79927398710\n4539578763621486\n1234567812345678"
def luhn(s):
    t = 0
    for i, ch in enumerate(reversed(s)):
        d = int(ch)
        if i % 2 == 1:
            d *= 2
            if d > 9: d -= 9
        t += d
    return t % 10 == 0
task("w32", "logic", "Read in/ids.txt (one digit string per line). Write out/result.txt with \"<digits> valid\" or \"<digits> invalid\" per line according to the Luhn checksum (from the rightmost digit, double every second digit, subtract 9 from results over 9, sum all; valid if the sum is divisible by 10).",
     {"in/ids.txt": ids}, "out/result.txt", "text", "\n".join(f"{x} {'valid' if luhn(x) else 'invalid'}" for x in L(ids)))
board = [{"name": "kim", "score": 88}, {"name": "ari", "score": 95}, {"name": "zed", "score": 88},
         {"name": "bea", "score": 70}, {"name": "lou", "score": 95}, {"name": "max", "score": 81}]
def ranks(b):
    srt = sorted(b, key=lambda r: (-r["score"], r["name"]))
    out, rank, prev = [], 0, None
    for r in srt:
        if r["score"] != prev: rank += 1; prev = r["score"]
        out.append(f"{rank} {r['name']} {r['score']}")
    return "\n".join(out)
task("w33", "logic", "Read in/board.json (array of {name, score}). Sort by score descending, then name ascending. Write out/result.txt with one line per entry \"<rank> <name> <score>\" using dense ranking (equal scores share a rank; the next distinct score gets the next rank).",
     {"in/board.json": json.dumps(board)}, "out/result.txt", "text", ranks(board))
maze = "#.#..\n..S#.\n.###.\n....."
def around(g):
    rows = L(g)
    for r, row in enumerate(rows):
        c = row.find("S")
        if c >= 0:
            n = 0
            for dr, dc in ((-1, 0), (1, 0), (0, -1), (0, 1)):
                rr, cc = r + dr, c + dc
                if 0 <= rr < len(rows) and 0 <= cc < len(rows[rr]) and rows[rr][cc] == "#": n += 1
            return f"{r},{c} {n}"
task("w34", "logic", "Read in/maze.txt (rows of '#', '.' and exactly one 'S'). Write out/result.txt with \"<row>,<col> <walls>\": the 0-based row and column of 'S' and the number of '#' cells directly up, down, left or right of it (cells off the grid do not count).",
     {"in/maze.txt": maze}, "out/result.txt", "text", around(maze))
prices = [14, 3, 9, 22, 6, 11]
def pair(xs, t):
    for j in range(len(xs)):
        for i in range(j):
            if xs[i] + xs[j] == t: return f"{i} {j}"
    return "none"
task("w35", "logic", "Read in/prices.json (array of integers) and in/target.txt (one integer). Find the pair of 0-based indices i < j with prices[i] + prices[j] == target that has the smallest j (then the smallest i). Write \"<i> <j>\" to out/result.txt, or \"none\" if no pair exists.",
     {"in/prices.json": json.dumps(prices), "in/target.txt": "20"}, "out/result.txt", "text", pair(prices, 20))
pw = "hunter2\nCorrectHorse\ncorrecthorse9\nBattery9Staple\nShort1"
def policy(p):
    if len(p) < 8: return "too short"
    if not any(c.isdigit() for c in p): return "no digit"
    if not any(c.isupper() for c in p): return "no upper"
    return "ok"
task("w36", "logic", "Read in/passwords.txt (one per line). For each, write the first failing rule, checked in this order: \"too short\" if fewer than 8 characters, \"no digit\" if it has no 0-9, \"no upper\" if it has no A-Z; otherwise \"ok\". One result per line in out/result.txt.",
     {"in/passwords.txt": pw}, "out/result.txt", "text", "\n".join(policy(p) for p in L(pw)))
ttt = "XOX\nOXO\nOXX"
def winner(g):
    b = L(g)
    lines = [row for row in b] + ["".join(b[r][c] for r in range(3)) for c in range(3)] + \
            ["".join(b[i][i] for i in range(3)), "".join(b[i][2 - i] for i in range(3))]
    for p in "XO":
        if p * 3 in lines: return p
    return "draw" if all("." not in row for row in b) else "open"
task("w37", "logic", "Read in/board.txt (3 lines of 3 characters, each 'X', 'O' or '.'). Write out/result.txt with \"X\" or \"O\" if that player has three in a row (row, column or diagonal), else \"draw\" if no '.' remains, else \"open\". At most one player has a line.",
     {"in/board.txt": ttt}, "out/result.txt", "text", winner(ttt))

Path(__file__).with_name("manifest.json").write_text(json.dumps({"version": "heldout-v2", "tasks": T}, indent=2) + "\n")
print(len(T), "tasks")
