import json
from pathlib import Path

people = json.loads(Path("in/people.json").read_text())
lines = [f"{p['id']} {'valid' if 0 <= p['age'] <= 150 else 'invalid'}" for p in people]
Path("out/result.txt").write_text("\n".join(lines) + "\n")
