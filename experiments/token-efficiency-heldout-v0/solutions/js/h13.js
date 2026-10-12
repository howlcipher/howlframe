const fs = require("fs");

const scores = JSON.parse(fs.readFileSync("in/scores.json", "utf8"));
const total = scores.reduce((a, b) => a + b, 0);
fs.writeFileSync("out/result.txt", `${Math.floor(total / scores.length)}\n`);
