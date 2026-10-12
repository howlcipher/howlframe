import json
from collections import Counter
from pathlib import Path

lines = Path("in/events.log").read_text().split("\n")
counts = Counter(line.split("|")[0] for line in lines)
Path("out/result.json").write_text(json.dumps(counts, indent=2) + "\n")
