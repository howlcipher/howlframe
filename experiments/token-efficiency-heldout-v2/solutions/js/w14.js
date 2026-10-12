const fs = require("fs");

const left = JSON.parse(fs.readFileSync("in/left.json", "utf8"));
const right = JSON.parse(fs.readFileSync("in/right.json", "utf8"));
const out = [];
for (let i = 0; i < Math.max(left.length, right.length); i++) out.push(...left.slice(i, i + 1), ...right.slice(i, i + 1));
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
