from pathlib import Path

rows = [line.split("\t") for line in Path("in/stock.tsv").read_text().split("\n")[1:]]
total = sum(int(qty) * int(price) for sku, qty, price in rows)
Path("out/result.txt").write_text(f"{total}\n")
