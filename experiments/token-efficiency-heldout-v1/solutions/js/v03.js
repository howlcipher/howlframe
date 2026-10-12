const fs = require("fs");

const lines = fs.readFileSync("in/poem.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.map((line) => line.slice(0, 3)).join("\n") + "\n");
