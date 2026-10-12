import json
from pathlib import Path

left = json.loads(Path("in/left.json").read_text())
right = json.loads(Path("in/right.json").read_text())
out = []
for i in range(max(len(left), len(right))):
    out += left[i:i + 1] + right[i:i + 1]
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
