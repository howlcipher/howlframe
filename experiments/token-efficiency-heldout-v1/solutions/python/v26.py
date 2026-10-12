import json
from pathlib import Path

orders = json.loads(Path("in/orders.json").read_text())
totals = {}
for order in orders:
    totals[order["cust"]] = totals.get(order["cust"], 0) + order["total"]
Path("out/result.json").write_text(json.dumps(totals, indent=2) + "\n")
