from pathlib import Path

lines = Path("in/nums.txt").read_text().split("\n")
Path("out/result.txt").write_text("\n".join(format(int(x), "b") for x in lines) + "\n")
