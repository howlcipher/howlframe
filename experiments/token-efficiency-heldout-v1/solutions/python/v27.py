import json
from pathlib import Path

orders = json.loads(Path("in/orders.json").read_text())
Path("out/result.txt").write_text("\n".join(order["id"] for order in orders if order["total"] >= 100) + "\n")
