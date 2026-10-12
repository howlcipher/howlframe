const fs = require("fs");

const lines = fs.readFileSync("in/events.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.filter((line, i) => i === 0 || line !== lines[i - 1]).join("\n") + "\n");
