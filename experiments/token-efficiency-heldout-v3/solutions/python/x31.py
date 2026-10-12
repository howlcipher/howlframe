from pathlib import Path

out = []
for item in Path("in/ranges.txt").read_text().strip().split(","):
    a, _, b = item.partition("-")
    out += range(int(a), int(b or a) + 1)
Path("out/result.txt").write_text(",".join(map(str, out)) + "\n")
