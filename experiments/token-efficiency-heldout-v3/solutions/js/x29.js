const fs = require("fs");

const v = JSON.parse(fs.readFileSync("in/visits.json", "utf8"));
const out = v.filter((x, i) => v.lastIndexOf(x) === i);
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
