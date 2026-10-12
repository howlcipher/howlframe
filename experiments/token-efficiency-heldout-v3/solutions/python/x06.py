import json
from pathlib import Path

out = {}
for s in json.loads(Path("in/sales.json").read_text()):
    r = out.setdefault(s["region"], {"units": 0, "products": []})
    r["units"] += s["units"]
    if s["product"] not in r["products"]:
        r["products"].append(s["product"])
for r in out.values():
    r["products"].sort()
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
