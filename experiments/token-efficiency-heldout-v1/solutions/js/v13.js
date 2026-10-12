const fs = require("fs");

const temps = fs.readFileSync("in/temps.txt", "utf8").trim().split(",").map(Number);
let total = 0;
fs.writeFileSync("out/result.json", JSON.stringify(temps.map((t) => (total += t)), null, 2) + "\n");
