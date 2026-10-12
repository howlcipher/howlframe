import json
from pathlib import Path

out = {}
for line in Path("in/requests.jsonl").read_text().splitlines():
    r = json.loads(line)
    out[r["level"]] = max(out.get(r["level"], r["ms"]), r["ms"])
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
