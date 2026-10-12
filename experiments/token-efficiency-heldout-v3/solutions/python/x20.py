from pathlib import Path

paras = [p for p in Path("in/doc.txt").read_text().split("\n\n") if p.strip()]
lines = [f"{i}: {len(p.split())}" for i, p in enumerate(paras, 1)]
Path("out/result.txt").write_text("\n".join(lines) + "\n")
