import os
from pathlib import Path

words = Path("in/words.txt").read_text().splitlines()
Path("out/result.txt").write_text(os.path.commonprefix(words) + "\n")
