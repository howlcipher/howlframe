const fs = require("fs");

const lines = fs.readFileSync("in/text.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.reverse().join("\n") + "\n");
