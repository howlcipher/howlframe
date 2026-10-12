from pathlib import Path

major, minor, _ = Path("in/version.txt").read_text().strip().split(".")
Path("out/result.txt").write_text(f"{major}.{int(minor) + 1}.0\n")
