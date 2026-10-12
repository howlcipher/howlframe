from collections import Counter
from pathlib import Path

counts = Counter(int(x) // 10 for x in Path("in/ages.txt").read_text().strip().split(","))
Path("out/result.txt").write_text("\n".join(f"{b * 10}-{b * 10 + 9}: {counts[b]}" for b in sorted(counts)) + "\n")
