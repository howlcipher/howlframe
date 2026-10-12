from pathlib import Path

path = Path("in/path.txt").read_text().strip()
Path("out/result.txt").write_text(path.rsplit(".", 1)[1] + "\n")
