import json
from pathlib import Path

queue = json.loads(Path("in/queue.json").read_text())
index = queue.index("build") if "build" in queue else -1
Path("out/result.txt").write_text(f"{index}\n")
