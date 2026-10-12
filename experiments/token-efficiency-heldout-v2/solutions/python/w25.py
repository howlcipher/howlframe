from pathlib import Path

n = int(Path("in/n.txt").read_text())
primes = [p for p in range(2, n + 1) if all(p % d for d in range(2, int(p**0.5) + 1))]
Path("out/result.txt").write_text(" ".join(map(str, primes)) + "\n")
