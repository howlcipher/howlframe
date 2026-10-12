import json
from pathlib import Path

prices = json.loads(Path("in/prices.json").read_text())
out = {k: f"${v // 100}.{v % 100:02d}" for k, v in prices.items()}
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
