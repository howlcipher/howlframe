import json
from pathlib import Path

doc = json.loads(Path("in/doc.json").read_text())
for op in json.loads(Path("in/ops.json").read_text()):
    *parts, last = op["path"].split(".")
    node = doc
    for p in parts:
        node = node.setdefault(p, {})
    if op["op"] == "set":
        node[last] = op["value"]
    else:
        node.pop(last, None)
Path("out/result.json").write_text(json.dumps(doc, indent=2) + "\n")
