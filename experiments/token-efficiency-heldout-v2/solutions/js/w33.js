const fs = require("fs");

const board = JSON.parse(fs.readFileSync("in/board.json", "utf8"));
board.sort((a, b) => b.score - a.score || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
const scores = [...new Set(board.map((e) => e.score))];
fs.writeFileSync("out/result.txt", board.map((e) => `${scores.indexOf(e.score) + 1} ${e.name} ${e.score}`).join("\n") + "\n");
