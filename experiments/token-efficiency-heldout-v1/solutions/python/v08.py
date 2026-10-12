from pathlib import Path

rows = [line.split("\t") for line in Path("in/stock.tsv").read_text().split("\n")[1:]]
best = max(rows, key=lambda row: int(row[2]))
Path("out/result.txt").write_text(best[0] + "\n")
