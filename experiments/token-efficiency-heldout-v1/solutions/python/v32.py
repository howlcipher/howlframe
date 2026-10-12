from pathlib import Path

text = Path("in/brackets.txt").read_text().strip()
pairs = {")": "(", "]": "[", "}": "{"}
stack = []
result = None
for i, ch in enumerate(text):
    if ch in "([{":
        stack.append(ch)
    elif ch in pairs and (not stack or stack.pop() != pairs[ch]):
        result = f"error at {i}"
        break
if result is None:
    result = "unclosed" if stack else "ok"
Path("out/result.txt").write_text(result + "\n")
