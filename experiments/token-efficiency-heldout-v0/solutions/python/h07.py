from pathlib import Path

lines = Path("in/data.csv").read_text().split("\n")
header = lines[0].split(",")
col = header.index("age")
kept = [row for row in lines[1:] if int(row.split(",")[col]) >= 30]
Path("out/result.csv").write_text("\n".join([lines[0]] + kept) + "\n")
