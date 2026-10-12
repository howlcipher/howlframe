import json
from pathlib import Path

stock = json.loads(Path("in/stock.json").read_text())
for m in json.loads(Path("in/moves.json").read_text()):
    stock[m["sku"]] = stock.get(m["sku"], 0) + m["delta"]
Path("out/result.json").write_text(json.dumps(stock, indent=2) + "\n")
