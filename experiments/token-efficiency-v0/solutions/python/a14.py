import json
from pathlib import Path

checks = json.loads(Path("evidence/checks.json").read_text())
version_text = Path("evidence/version.kv").read_text()
Path("evidence/note.txt").read_text()
selected = [check for check in checks if check["name"] == "unit"]
record = {"batch": "14", "selected": selected}
versions = dict(line.split("=", 1) for line in version_text.splitlines())
current = tuple(map(int, versions["current"].split(".")))
proposed = tuple(map(int, versions["proposed"].split(".")))
if all(check["status"] == "PASS" for check in checks) and proposed > current:
    path = "out/release.json"
else:
    path = "out/approval_request.json"
    record = {"reason": "blocked", "summary": {}}
Path(path).write_text(json.dumps(record, indent=2) + "\n")
