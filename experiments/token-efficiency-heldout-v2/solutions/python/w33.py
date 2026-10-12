import json
from pathlib import Path

board = sorted(json.loads(Path("in/board.json").read_text()), key=lambda e: (-e["score"], e["name"]))
scores = sorted({e["score"] for e in board}, reverse=True)
Path("out/result.txt").write_text("\n".join(f"{scores.index(e['score']) + 1} {e['name']} {e['score']}" for e in board) + "\n")
