import json
from pathlib import Path

ring = json.loads(Path("in/ring.json").read_text())
k = int(Path("in/k.txt").read_text()) % len(ring)
Path("out/result.json").write_text(json.dumps(ring[k:] + ring[:k], indent=2) + "\n")
