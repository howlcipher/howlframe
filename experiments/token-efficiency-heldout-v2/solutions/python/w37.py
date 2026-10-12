from pathlib import Path

b = Path("in/board.txt").read_text().replace("\n", "")
lines = [(0, 1, 2), (3, 4, 5), (6, 7, 8), (0, 3, 6), (1, 4, 7), (2, 5, 8), (0, 4, 8), (2, 4, 6)]
winners = [b[i] for i, j, k in lines if b[i] != "." and b[i] == b[j] == b[k]]
Path("out/result.txt").write_text((winners[0] if winners else "open" if "." in b else "draw") + "\n")
