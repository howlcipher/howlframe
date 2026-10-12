from pathlib import Path

lines = Path("in/text.txt").read_text().split("\n")
Path("out/result.txt").write_text("\n".join(reversed(lines)) + "\n")
