from pathlib import Path

count = sum(1 for path in Path("in/docs").rglob("*") if path.is_file())
Path("out/result.txt").write_text(f"{count}\n")
