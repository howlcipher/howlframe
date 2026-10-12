from pathlib import Path

rows = [line.split("\t") for line in Path("in/stock.tsv").read_text().split("\n")[1:]]
Path("out/result.txt").write_text("\n".join(sku for sku, qty, price in rows if int(qty) == 0) + "\n")
