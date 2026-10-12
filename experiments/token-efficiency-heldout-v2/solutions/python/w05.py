import json
from pathlib import Path

out = {}
for line in Path("in/ledger.csv").read_text().split("\n")[1:]:
    _, account, amount = line.split(",")
    out[account] = out.get(account, 0) + int(amount)
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
