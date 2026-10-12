const fs = require("fs");

const lines = fs.readFileSync("in/data.csv", "utf8").split("\n");
const col = lines[0].split(",").indexOf("age");
const kept = lines.slice(1).filter((row) => parseInt(row.split(",")[col], 10) >= 30);
fs.writeFileSync("out/result.csv", [lines[0], ...kept].join("\n") + "\n");
