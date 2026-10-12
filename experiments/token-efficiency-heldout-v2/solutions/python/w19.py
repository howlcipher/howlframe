from pathlib import Path

msg = Path("in/msg.txt").read_text().strip()
Path("out/result.txt").write_text("".join(chr((ord(c) - 97 + 3) % 26 + 97) if "a" <= c <= "z" else c for c in msg) + "\n")
