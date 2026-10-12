import json
from pathlib import Path

def flat(v):
    return [x for e in v for x in flat(e)] if isinstance(v, list) else [v]


deep = json.loads(Path("in/deep.json").read_text())
Path("out/result.json").write_text(json.dumps(flat(deep), indent=2) + "\n")
