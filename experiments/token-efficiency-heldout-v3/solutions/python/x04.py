import json
from pathlib import Path

out = {}
for k, v in json.loads(Path("in/flat.json").read_text()).items():
    *parts, last = k.split(".")
    node = out
    for p in parts:
        node = node.setdefault(p, {})
    node[last] = v
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
