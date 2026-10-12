import json
from pathlib import Path

def flat(d, pre, out):
    for k, v in d.items():
        if isinstance(v, dict) and v:
            flat(v, pre + k + ".", out)
        else:
            out[pre + k] = v
    return out


d = json.loads(Path("in/nested.json").read_text())
Path("out/result.json").write_text(json.dumps(flat(d, "", {}), indent=2) + "\n")
