import json
from pathlib import Path

old = json.loads(Path("in/old.json").read_text())
new = json.loads(Path("in/new.json").read_text())
out = {
    "added": sorted(k for k in new if k not in old),
    "removed": sorted(k for k in old if k not in new),
    "changed": sorted(k for k in old if k in new and old[k] != new[k]),
}
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
