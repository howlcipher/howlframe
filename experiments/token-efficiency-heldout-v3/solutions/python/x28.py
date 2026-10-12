import json
from pathlib import Path

a = json.loads(Path("in/a.json").read_text())
b = json.loads(Path("in/b.json").read_text())
out = {"only_a": list(dict.fromkeys(x for x in a if x not in b)),
       "only_b": list(dict.fromkeys(x for x in b if x not in a))}
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
