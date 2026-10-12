const fs = require("fs");

const lines = fs.readFileSync("in/cards.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.map((line) => "*".repeat(line.length - 4) + line.slice(-4)).join("\n") + "\n");
