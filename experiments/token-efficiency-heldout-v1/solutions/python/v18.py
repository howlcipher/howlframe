import json
from pathlib import Path

queue = json.loads(Path("in/queue.json").read_text())
Path("out/result.json").write_text(json.dumps(sorted(set(queue)), indent=2) + "\n")
