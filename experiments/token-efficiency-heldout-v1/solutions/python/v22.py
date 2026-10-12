from pathlib import Path

word = Path("in/word.txt").read_text().strip()
Path("out/result.txt").write_text(("yes" if word == word[::-1] else "no") + "\n")
