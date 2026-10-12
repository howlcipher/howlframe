import json
from pathlib import Path

lines = Path("in/config.kv").read_text().split("\n")
config = dict(line.split("=", 1) for line in lines)
Path("out/result.json").write_text(json.dumps(config, indent=2) + "\n")
