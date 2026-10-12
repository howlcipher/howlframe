const fs = require("fs");

const lines = fs.readFileSync("in/text.txt", "utf8").split("\n");
const longest = lines.reduce((best, line) => (line.length > best.length ? line : best));
fs.writeFileSync("out/result.txt", longest + "\n");
