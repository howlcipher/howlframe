from pathlib import Path

rows = [l.split(",") for l in Path("in/grid.csv").read_text().splitlines()]
Path("out/result.csv").write_text("\n".join(",".join(c) for c in zip(*rows)) + "\n")
