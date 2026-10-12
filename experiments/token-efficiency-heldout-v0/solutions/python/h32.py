from pathlib import Path

root = Path("in/docs")
paths = sorted(p.relative_to(root).as_posix() for p in root.rglob("*.txt") if p.is_file())
Path("out/result.txt").write_text("\n".join(paths) + "\n")
