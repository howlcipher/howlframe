const fs = require("fs");

const m = JSON.parse(fs.readFileSync("in/meter.json", "utf8"));
fs.writeFileSync("out/result.json", JSON.stringify(m.slice(1).map((v, i) => v - m[i]), null, 2) + "\n");
