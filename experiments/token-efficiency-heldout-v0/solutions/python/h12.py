import json
from collections import Counter
from pathlib import Path

text = Path("in/words.txt").read_text().lower()
counts = Counter(word for line in text.split("\n") for word in line.split(" "))
Path("out/result.json").write_text(json.dumps(counts, indent=2) + "\n")
