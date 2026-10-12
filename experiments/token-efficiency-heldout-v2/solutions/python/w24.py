import json
from pathlib import Path

out = {}
for cart in json.loads(Path("in/carts.json").read_text()):
    for item in cart["items"]:
        out[item["sku"]] = out.get(item["sku"], 0) + item["qty"]
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
