const fs = require("fs");

const raw = fs.readFileSync("in/raw.txt", "utf8").trim();
const runs = raw.match(/(.)\1*/g);
fs.writeFileSync("out/result.txt", runs.map((r) => r[0] + r.length).join("") + "\n");
