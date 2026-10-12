from pathlib import Path

words = Path("in/sentence.txt").read_text().strip().split(" ")
Path("out/result.txt").write_text(max(words, key=len) + "\n")
