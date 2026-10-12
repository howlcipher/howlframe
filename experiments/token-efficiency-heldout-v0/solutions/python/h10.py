import json
from collections import Counter
from pathlib import Path

lines = Path("in/data.csv").read_text().split("\n")
col = lines[0].split(",").index("dept")
counts = Counter(row.split(",")[col] for row in lines[1:])
Path("out/result.json").write_text(json.dumps(counts, indent=2) + "\n")
