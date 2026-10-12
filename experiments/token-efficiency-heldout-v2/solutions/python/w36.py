from pathlib import Path


def check(p):
    if len(p) < 8:
        return "too short"
    if not any(c.isdigit() for c in p):
        return "no digit"
    if not any("A" <= c <= "Z" for c in p):
        return "no upper"
    return "ok"


lines = Path("in/passwords.txt").read_text().split("\n")
Path("out/result.txt").write_text("\n".join(check(p) for p in lines) + "\n")
