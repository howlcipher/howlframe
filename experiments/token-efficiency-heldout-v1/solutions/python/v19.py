import json
from collections import Counter
from pathlib import Path

queue = json.loads(Path("in/queue.json").read_text())
Path("out/result.json").write_text(json.dumps(Counter(queue), indent=2) + "\n")
