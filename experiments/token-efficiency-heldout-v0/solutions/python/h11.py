import json
from pathlib import Path

numbers = [int(line) for line in Path("in/numbers.txt").read_text().split("\n")]
result = {"min": min(numbers), "max": max(numbers), "sum": sum(numbers)}
Path("out/result.json").write_text(json.dumps(result, indent=2) + "\n")
