const fs = require("fs");

const lines = fs.readFileSync("in/text.txt", "utf8").split("\n");
const numbered = lines.map((line, i) => `${i + 1}: ${line}`);
fs.writeFileSync("out/result.txt", numbered.join("\n") + "\n");
