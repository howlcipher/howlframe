import json
from pathlib import Path

def merge(a, b):
    for k, v in b.items():
        a[k] = merge(a[k], v) if isinstance(a.get(k), dict) and isinstance(v, dict) else v
    return a


base = json.loads(Path("in/base.json").read_text())
over = json.loads(Path("in/override.json").read_text())
Path("out/result.json").write_text(json.dumps(merge(base, over), indent=2) + "\n")
