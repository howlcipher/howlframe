from pathlib import Path

lines = []
for l in Path("in/pairs.txt").read_text().splitlines():
    a, b = l.split()
    lines.append("yes" if sorted(a) == sorted(b) else "no")
Path("out/result.txt").write_text("\n".join(lines) + "\n")
