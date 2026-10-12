const fs = require("fs");

const temps = fs.readFileSync("in/temps.txt", "utf8").trim().split(",").map(Number);
fs.writeFileSync("out/result.json", JSON.stringify(temps.map((t) => t * 2), null, 2) + "\n");
