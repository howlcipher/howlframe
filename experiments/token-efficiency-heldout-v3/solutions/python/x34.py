import datetime
from pathlib import Path

lines = [(datetime.date(2021, 1, 1) + datetime.timedelta(int(d) - 1)).strftime("%m-%d")
         for d in Path("in/days.txt").read_text().split()]
Path("out/result.txt").write_text("\n".join(lines) + "\n")
