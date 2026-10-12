const fs = require("fs");

const temps = fs.readFileSync("in/temps.txt", "utf8").trim().split(",").map(Number);
fs.writeFileSync("out/result.txt", String(temps.indexOf(Math.max(...temps)) + 1) + "\n");
