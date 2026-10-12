const fs = require("fs");

const lines = fs.readFileSync("in/poem.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.filter((line) => line.split(" ").length === 3).join("\n") + "\n");
