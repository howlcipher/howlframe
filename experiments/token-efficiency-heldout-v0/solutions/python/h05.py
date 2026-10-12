from pathlib import Path

text = Path("in/text.txt").read_text()
Path("out/result.txt").write_text(text.replace("foo", "bar") + "\n")
