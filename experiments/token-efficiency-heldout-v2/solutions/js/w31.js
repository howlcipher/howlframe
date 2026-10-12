const fs = require("fs");

const lines = fs.readFileSync("in/nums.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.map((x) => Number(x).toString(2)).join("\n") + "\n");
