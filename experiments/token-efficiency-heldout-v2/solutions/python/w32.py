from pathlib import Path


def luhn(digits):
    total = 0
    for i, c in enumerate(reversed(digits)):
        d = int(c) * (2 if i % 2 else 1)
        total += d - 9 if d > 9 else d
    return total % 10 == 0


lines = Path("in/ids.txt").read_text().split("\n")
Path("out/result.txt").write_text("\n".join(f"{x} {'valid' if luhn(x) else 'invalid'}" for x in lines) + "\n")
