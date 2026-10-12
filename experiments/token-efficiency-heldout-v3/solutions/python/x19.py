from pathlib import Path

out, inside = [], False
for line in Path("in/app.ini").read_text().splitlines():
    if line.startswith("["):
        inside = line == "[net]"
    elif inside and line:
        out.append(line)
Path("out/result.txt").write_text("\n".join(out) + "\n")
