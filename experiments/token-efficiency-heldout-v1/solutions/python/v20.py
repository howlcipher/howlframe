import json
from pathlib import Path

queue = json.loads(Path("in/queue.json").read_text())
Path("out/result.json").write_text(json.dumps(queue[1:-1], indent=2) + "\n")
