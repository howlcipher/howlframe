from pathlib import Path

lines = Path("in/headings.txt").read_text().split("\n")
Path("out/result.txt").write_text("\n".join(" ".join(w[:1].upper() + w[1:] for w in line.split(" ")) for line in lines) + "\n")
