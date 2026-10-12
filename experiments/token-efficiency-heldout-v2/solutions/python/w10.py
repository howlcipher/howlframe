from pathlib import Path

out = []
for x in Path("in/seconds.txt").read_text().split("\n"):
    s = int(x)
    out.append(f"{s // 3600}:{s % 3600 // 60:02d}:{s % 60:02d}")
Path("out/result.txt").write_text("\n".join(out) + "\n")
