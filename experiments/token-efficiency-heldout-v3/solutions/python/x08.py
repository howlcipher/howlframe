import json
from pathlib import Path

doc = json.loads(Path("in/doc.json").read_text())
Path("out/result.txt").write_text(json.dumps(doc, sort_keys=True, separators=(",", ":")) + "\n")
