from pathlib import Path

lines = []
for n in Path("in/nums.txt").read_text().split():
    d = int(n)
    while d > 9:
        d = sum(map(int, str(d)))
    lines.append(f"{n} -> {d}")
Path("out/result.txt").write_text("\n".join(lines) + "\n")
