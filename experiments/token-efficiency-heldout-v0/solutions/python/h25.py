from pathlib import Path

values = [int(line) for line in Path("in/seq.txt").read_text().split("\n")]
result = "sorted"
for i in range(1, len(values)):
    if values[i] < values[i - 1]:
        result = f"unsorted at {i + 1}"
        break
Path("out/result.txt").write_text(result + "\n")
