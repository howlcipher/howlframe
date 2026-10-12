const fs = require("fs");

const lines = fs.readFileSync("in/poem.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.sort().join("\n") + "\n");
