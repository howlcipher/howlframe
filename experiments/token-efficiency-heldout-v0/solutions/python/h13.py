import json
from pathlib import Path

scores = json.loads(Path("in/scores.json").read_text())
Path("out/result.txt").write_text(f"{sum(scores) // len(scores)}\n")
