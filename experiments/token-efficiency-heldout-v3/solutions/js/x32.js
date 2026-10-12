const fs = require("fs");

const words = fs.readFileSync("in/words.txt", "utf8").trim().split("\n");
let p = words[0];
for (const w of words) while (!w.startsWith(p)) p = p.slice(0, -1);
fs.writeFileSync("out/result.txt", p + "\n");
