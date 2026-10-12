import json
from pathlib import Path

env = json.loads(Path("in/env.json").read_text())
out = [{"key": k, "value": env[k]} for k in sorted(env)]
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
