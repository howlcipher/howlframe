import json
from pathlib import Path

jobs = json.loads(Path("in/jobs.json").read_text())
Path("out/result.json").write_text(json.dumps([jobs[i:i + 3] for i in range(0, len(jobs), 3)], indent=2) + "\n")
