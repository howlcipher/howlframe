from pathlib import Path

lines = Path("in/poem.txt").read_text().split("\n")
Path("out/result.txt").write_text("\n".join(sorted(lines)) + "\n")
