import json
from pathlib import Path

prices = json.loads(Path("in/prices.json").read_text())
target = int(Path("in/target.txt").read_text())
pairs = [(i, j) for j in range(len(prices)) for i in range(j) if prices[i] + prices[j] == target]
Path("out/result.txt").write_text((f"{pairs[0][0]} {pairs[0][1]}" if pairs else "none") + "\n")
