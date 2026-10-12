from pathlib import Path

budget = int(Path("in/budget.txt").read_text())
result = "within budget"
total = 0
for n, x in enumerate(Path("in/spend.txt").read_text().split("\n"), 1):
    total += int(x)
    if total > budget:
        result = f"over at {n}"
        break
Path("out/result.txt").write_text(result + "\n")
