import json
from pathlib import Path

checks = json.loads(Path("evidence/checks.json").read_text())
version_text = Path("evidence/version.kv").read_text()
Path("evidence/note.txt").read_text()
selected = [check for check in checks if check["status"] == "PASS"]
record = {"batch": "03", "selected": selected}
path = "out/approval_request.json"
record = {"reason": "release review", "summary": record}
Path(path).write_text(json.dumps(record, indent=2) + "\n")
