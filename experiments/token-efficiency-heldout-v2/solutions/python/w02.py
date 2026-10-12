from pathlib import Path

out = []
for line in Path("in/events.txt").read_text().split("\n"):
    if not out or out[-1] != line:
        out.append(line)
Path("out/result.txt").write_text("\n".join(out) + "\n")
