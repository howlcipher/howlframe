const fs = require("fs");

const a = JSON.parse(fs.readFileSync("in/a.json", "utf8"));
const b = JSON.parse(fs.readFileSync("in/b.json", "utf8"));
const out = { only_a: [...new Set(a.filter((x) => !b.includes(x)))], only_b: [...new Set(b.filter((x) => !a.includes(x)))] };
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
