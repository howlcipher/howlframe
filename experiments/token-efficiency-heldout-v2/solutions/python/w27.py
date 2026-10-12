from pathlib import Path

out = []
for line in Path("in/years.txt").read_text().split("\n"):
    y = int(line)
    leap = y % 4 == 0 and (y % 100 != 0 or y % 400 == 0)
    out.append(f"{y} {'leap' if leap else 'common'}")
Path("out/result.txt").write_text("\n".join(out) + "\n")
