from pathlib import Path

total = 0
for line in Path("in/times.txt").read_text().split("\n"):
    hours, minutes = line.split(":")
    total += int(hours) * 60 + int(minutes)
Path("out/result.txt").write_text(f"{total}\n")
