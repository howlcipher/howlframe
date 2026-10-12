from pathlib import Path

lines = Path("in/data.csv").read_text().split("\n")
col = lines[0].split(",").index("amount")
total = sum(int(row.split(",")[col]) for row in lines[1:])
Path("out/result.txt").write_text(f"{total}\n")
