const fs = require("fs");

const temps = fs.readFileSync("in/temps.txt", "utf8").trim().split(",").map(Number);
fs.writeFileSync("out/result.json", JSON.stringify(temps.filter((t) => t > temps[0]), null, 2) + "\n");
