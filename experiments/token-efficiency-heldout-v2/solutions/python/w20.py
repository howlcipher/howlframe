from pathlib import Path

lines = Path("in/cards.txt").read_text().split("\n")
Path("out/result.txt").write_text("\n".join("*" * (len(line) - 4) + line[-4:] for line in lines) + "\n")
