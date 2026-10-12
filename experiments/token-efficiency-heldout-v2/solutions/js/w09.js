const fs = require("fs");

const values = fs.readFileSync("in/latency.txt", "utf8").split("\n").map(Number).sort((a, b) => a - b);
fs.writeFileSync("out/result.txt", values[Math.floor(values.length / 2)] + "\n");
