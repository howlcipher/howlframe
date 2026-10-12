from pathlib import Path

lines = Path("in/lines.csv").read_text().split("\n")
out = [lines[0] + ",total"]
for line in lines[1:]:
    _, qty, unit = line.split(",")
    out.append(f"{line},{int(qty) * int(unit)}")
Path("out/result.csv").write_text("\n".join(out) + "\n")
