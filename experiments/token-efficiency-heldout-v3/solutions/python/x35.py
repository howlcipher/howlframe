import json
from pathlib import Path

deps = json.loads(Path("in/deps.json").read_text())
out = []
while len(out) < len(deps):
    out.append(min(t for t in deps if t not in out and all(d in out for d in deps[t])))
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
