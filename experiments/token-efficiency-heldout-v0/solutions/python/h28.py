import json
from pathlib import Path

members = json.loads(Path("in/members.json").read_text())
teams = {}
for m in members:
    teams.setdefault(m["team"], []).append(m["member"])
Path("out/result.json").write_text(json.dumps(teams, indent=2) + "\n")
