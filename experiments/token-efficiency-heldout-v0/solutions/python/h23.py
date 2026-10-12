import json
from pathlib import Path

ids = json.loads(Path("in/ids.json").read_text())
result = "ok"
seen = set()
for item in ids:
    if item in seen:
        result = f"duplicate:{item}"
        break
    seen.add(item)
Path("out/result.txt").write_text(result + "\n")
