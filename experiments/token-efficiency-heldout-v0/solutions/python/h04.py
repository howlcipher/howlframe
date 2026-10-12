from pathlib import Path

lines = Path("in/text.txt").read_text().split("\n")
kept = [line.strip() for line in lines if line.strip()]
Path("out/result.txt").write_text("\n".join(kept) + "\n")
