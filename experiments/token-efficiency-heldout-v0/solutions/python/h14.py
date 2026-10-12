from pathlib import Path

lines = Path("in/app.log").read_text().split("\n")
count = sum(1 for line in lines if "ERROR" in line)
Path("out/result.txt").write_text(f"{count}\n")
