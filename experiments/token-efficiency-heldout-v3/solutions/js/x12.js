const fs = require("fs");

const out = {};
for (const p of JSON.parse(fs.readFileSync("in/people.json", "utf8"))) out[`${p.last}, ${p.first}`] = p.age;
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
