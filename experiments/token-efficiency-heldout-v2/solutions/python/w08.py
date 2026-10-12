from pathlib import Path

rows = [line.split(",") for line in Path("in/team.csv").read_text().split("\n")[1:]]
width = max(len(name) for name, _ in rows)
Path("out/result.txt").write_text("\n".join(f"{name.ljust(width)} | {role}" for name, role in rows) + "\n")
