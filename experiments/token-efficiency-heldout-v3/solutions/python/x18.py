from pathlib import Path

out, cur = [], ""
for line in Path("in/script.txt").read_text().splitlines():
    if line.endswith("\\"):
        cur += line[:-1]
    else:
        out.append(cur + line)
        cur = ""
Path("out/result.txt").write_text("\n".join(out) + "\n")
