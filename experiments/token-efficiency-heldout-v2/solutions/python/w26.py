from pathlib import Path

n = int(Path("in/n.txt").read_text())
steps = 0
while n != 1:
    n = n // 2 if n % 2 == 0 else 3 * n + 1
    steps += 1
Path("out/result.txt").write_text(f"{steps}\n")
