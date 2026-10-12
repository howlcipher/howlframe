import json
from pathlib import Path

orders = json.loads(Path("in/orders.json").read_text())
custs = list(dict.fromkeys(order["cust"] for order in orders))
Path("out/result.json").write_text(json.dumps(custs, indent=2) + "\n")
