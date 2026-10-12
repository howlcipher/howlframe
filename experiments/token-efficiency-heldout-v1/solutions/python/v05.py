from pathlib import Path

lines = Path("in/poem.txt").read_text().split("\n")
Path("out/result.txt").write_text("\n".join(line for line in lines if len(line.split(" ")) == 3) + "\n")
