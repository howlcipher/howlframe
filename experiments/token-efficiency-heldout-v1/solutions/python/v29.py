import json
from pathlib import Path

orders = json.loads(Path("in/orders.json").read_text())
cheapest = min(orders, key=lambda order: order["total"])
Path("out/result.txt").write_text(cheapest["id"] + "\n")
