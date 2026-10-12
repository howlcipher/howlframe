const fs = require("fs");

const a = fs.readFileSync("in/load.txt", "utf8").trim().split(",").map(Number);
const out = [];
for (let i = 0; i + 3 <= a.length; i++) out.push(Math.floor((a[i] + a[i + 1] + a[i + 2]) / 3));
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
