import json
from pathlib import Path

out = {"present": [], "null": [], "absent": []}
for c in json.loads(Path("in/contacts.json").read_text()):
    key = "absent" if "email" not in c else "null" if c["email"] is None else "present"
    out[key].append(c["id"])
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
