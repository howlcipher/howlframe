import json
from pathlib import Path

lines = []
for a in json.loads(Path("in/accounts.json").read_text()):
    city = a["user"].get("profile", {}).get("city")
    lines.append(f"{a['login']}: {city if city is not None else 'unknown'}")
Path("out/result.txt").write_text("\n".join(lines) + "\n")
