import json
from pathlib import Path

MISSING = object()
cfg = json.loads(Path("in/config.json").read_text())
out = []
for line in Path("in/paths.txt").read_text().splitlines():
    v = cfg
    for k in line.split("."):
        v = v[k] if isinstance(v, dict) and k in v else MISSING
    out.append("missing" if v is MISSING else json.dumps(v))
Path("out/result.txt").write_text("\n".join(out) + "\n")
