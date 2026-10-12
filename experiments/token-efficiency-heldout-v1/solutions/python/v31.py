from pathlib import Path

n = int(Path("in/n.txt").read_text().strip())
out = []
for i in range(1, n + 1):
    if i % 15 == 0:
        out.append("FizzBuzz")
    elif i % 3 == 0:
        out.append("Fizz")
    elif i % 5 == 0:
        out.append("Buzz")
    else:
        out.append(str(i))
Path("out/result.txt").write_text("\n".join(out) + "\n")
