from pathlib import Path

lines = Path("in/data.csv").read_text().split("\n")
header = lines[0].split(",")
col = header.index("email")
emails = [row.split(",")[col] for row in lines[1:]]
Path("out/result.txt").write_text("\n".join(emails) + "\n")
