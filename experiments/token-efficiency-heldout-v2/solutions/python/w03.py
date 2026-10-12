from pathlib import Path

lines = [line.split("#")[0].rstrip(" ") for line in Path("in/settings.txt").read_text().split("\n")]
Path("out/result.txt").write_text("\n".join(line for line in lines if line) + "\n")
