from pathlib import Path

grid = Path("in/maze.txt").read_text().split("\n")
r = next(i for i, row in enumerate(grid) if "S" in row)
c = grid[r].index("S")
walls = 0
for y, x in ((r - 1, c), (r + 1, c), (r, c - 1), (r, c + 1)):
    if 0 <= y < len(grid) and 0 <= x < len(grid[y]) and grid[y][x] == "#":
        walls += 1
Path("out/result.txt").write_text(f"{r},{c} {walls}\n")
