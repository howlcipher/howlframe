import json
from pathlib import Path

rows = [line.split("\t") for line in Path("in/stock.tsv").read_text().split("\n")[1:]]
stock = {sku: int(qty) for sku, qty, price in rows}
Path("out/result.json").write_text(json.dumps(stock, indent=2) + "\n")
