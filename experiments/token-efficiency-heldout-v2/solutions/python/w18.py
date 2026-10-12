from pathlib import Path

lines = Path("in/idents.txt").read_text().split("\n")
Path("out/result.txt").write_text("\n".join("".join("_" + c.lower() if c.isupper() else c for c in line) for line in lines) + "\n")
