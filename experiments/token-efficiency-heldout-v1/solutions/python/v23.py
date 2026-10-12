from pathlib import Path

title = Path("in/title.txt").read_text().strip().lower()
Path("out/result.txt").write_text("-".join(p for p in title.split(" ") if p) + "\n")
