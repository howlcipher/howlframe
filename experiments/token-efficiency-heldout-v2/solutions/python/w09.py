from pathlib import Path

values = sorted(int(x) for x in Path("in/latency.txt").read_text().split("\n"))
Path("out/result.txt").write_text(f"{values[len(values) // 2]}\n")
