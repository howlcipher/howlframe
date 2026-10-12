import json
from pathlib import Path

users = json.loads(Path("in/users.json").read_text())
scores = json.loads(Path("in/scores.json").read_text())
lines = [f"{u['name']} {sum(s['points'] for s in scores if s['user'] == u['id'])}" for u in users]
Path("out/result.txt").write_text("\n".join(lines) + "\n")
