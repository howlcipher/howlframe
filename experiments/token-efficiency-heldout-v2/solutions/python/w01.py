from pathlib import Path

out, cur = [], ""
for word in Path("in/para.txt").read_text().split():
    if not cur:
        cur = word
    elif len(cur) + 1 + len(word) <= 20:
        cur += " " + word
    else:
        out.append(cur)
        cur = word
out.append(cur)
Path("out/result.txt").write_text("\n".join(out) + "\n")
