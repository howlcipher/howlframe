import json
from pathlib import Path

orders = json.loads(Path("in/orders.json").read_text())
summary = {"count": len(orders), "total": sum(order["total"] for order in orders)}
Path("out/result.json").write_text(json.dumps(summary, indent=2) + "\n")
