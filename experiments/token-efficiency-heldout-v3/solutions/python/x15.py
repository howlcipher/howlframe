import json
from pathlib import Path

def depth(v):
    if isinstance(v, dict):
        v = list(v.values())
    if isinstance(v, list):
        return 1 + max(map(depth, v), default=0)
    return 0


tree = json.loads(Path("in/tree.json").read_text())
Path("out/result.txt").write_text(f"{depth(tree)}\n")
