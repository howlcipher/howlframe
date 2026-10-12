from pathlib import Path

lines = Path("in/sites.csv").read_text().splitlines()
out, prev = [lines[0]], []
for line in lines[1:]:
    prev = [c or p for c, p in zip(line.split(","), prev)] if prev else line.split(",")
    out.append(",".join(prev))
Path("out/result.csv").write_text("\n".join(out) + "\n")
