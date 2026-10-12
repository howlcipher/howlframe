import json
from pathlib import Path

schema = json.loads(Path("in/schema.json").read_text())
config = json.loads(Path("in/config.json").read_text())
missing = [key for key in schema if key not in config]
Path("out/result.json").write_text(json.dumps(missing, indent=2) + "\n")
