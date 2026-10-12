const fs = require("fs");

const grid = fs.readFileSync("in/maze.txt", "utf8").split("\n");
const r = grid.findIndex((row) => row.includes("S"));
const c = grid[r].indexOf("S");
const walls = [[r - 1, c], [r + 1, c], [r, c - 1], [r, c + 1]].filter(([y, x]) => grid[y] && grid[y][x] === "#").length;
fs.writeFileSync("out/result.txt", `${r},${c} ${walls}\n`);
