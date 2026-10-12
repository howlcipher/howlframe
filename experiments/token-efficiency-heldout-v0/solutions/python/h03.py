from pathlib import Path

lines = Path("in/text.txt").read_text().split("\n")
numbered = [f"{i}: {line}" for i, line in enumerate(lines, start=1)]
Path("out/result.txt").write_text("\n".join(numbered) + "\n")
