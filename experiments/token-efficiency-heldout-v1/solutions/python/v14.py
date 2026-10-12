from pathlib import Path

temps = [int(x) for x in Path("in/temps.txt").read_text().strip().split(",")]
Path("out/result.txt").write_text(f"{temps.index(max(temps)) + 1}\n")
