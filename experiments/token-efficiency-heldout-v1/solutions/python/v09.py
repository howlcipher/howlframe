from pathlib import Path

data = Path("in/stock.tsv").read_text().split("\n")[1:]
Path("out/result.txt").write_text("\n".join(sorted(data, key=lambda row: row.split("\t")[0])) + "\n")
