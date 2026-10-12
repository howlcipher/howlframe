import json
from pathlib import Path

users = json.loads(Path("in/users.json").read_text())
missing = [user["id"] for user in users if "email" not in user]
Path("out/result.json").write_text(json.dumps(missing, indent=2) + "\n")
