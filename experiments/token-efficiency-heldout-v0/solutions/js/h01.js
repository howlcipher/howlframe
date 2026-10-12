const fs = require("fs");

const lines = fs.readFileSync("in/text.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.map((line) => line.toUpperCase()).join("\n") + "\n");
