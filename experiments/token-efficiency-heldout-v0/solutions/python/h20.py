import json
from pathlib import Path

tags = Path("in/tags.txt").read_text().split(";")
unique = list(dict.fromkeys(tags))
Path("out/result.json").write_text(json.dumps(unique, indent=2) + "\n")
