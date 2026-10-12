import json
from pathlib import Path

codes = json.loads(Path("in/codes.json").read_text())
inverted = {value: key for key, value in codes.items()}
Path("out/result.json").write_text(json.dumps(inverted, indent=2) + "\n")
