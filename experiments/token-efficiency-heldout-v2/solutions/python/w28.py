from pathlib import Path

moves = {("closed", "open"): "opened", ("opened", "close"): "closed", ("closed", "lock"): "locked", ("locked", "unlock"): "closed"}
state = "closed"
result = None
for n, cmd in enumerate(Path("in/cmds.txt").read_text().split("\n"), 1):
    if (state, cmd) not in moves:
        result = f"error line {n}: cannot {cmd} when {state}"
        break
    state = moves[state, cmd]
Path("out/result.txt").write_text((result or state) + "\n")
