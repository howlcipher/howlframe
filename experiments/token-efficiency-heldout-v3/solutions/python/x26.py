from pathlib import Path

rows = [l.split() for l in Path("in/votes.txt").read_text().splitlines()]
total = sum(int(c) for _, c in rows)
Path("out/result.txt").write_text("\n".join(f"{n} {int(c) * 100 // total}%" for n, c in rows) + "\n")
