import json
from pathlib import Path

lines = Path("in/data.csv").read_text().split("\n")
header = lines[0].split(",")
rows = [dict(zip(header, row.split(","))) for row in lines[1:]]
Path("out/result.json").write_text(json.dumps(rows, indent=2) + "\n")
