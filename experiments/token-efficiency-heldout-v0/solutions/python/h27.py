import json
from pathlib import Path

user = json.loads(Path("in/profile.json").read_text())["user"]
result = {"name": user["name"], "tag_count": len(user["tags"])}
Path("out/result.json").write_text(json.dumps(result, indent=2) + "\n")
