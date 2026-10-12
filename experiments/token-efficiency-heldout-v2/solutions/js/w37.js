const fs = require("fs");

const b = fs.readFileSync("in/board.txt", "utf8").replace(/\n/g, "");
const lines = [[0, 1, 2], [3, 4, 5], [6, 7, 8], [0, 3, 6], [1, 4, 7], [2, 5, 8], [0, 4, 8], [2, 4, 6]];
const win = lines.find(([i, j, k]) => b[i] !== "." && b[i] === b[j] && b[j] === b[k]);
fs.writeFileSync("out/result.txt", (win ? b[win[0]] : b.includes(".") ? "open" : "draw") + "\n");
