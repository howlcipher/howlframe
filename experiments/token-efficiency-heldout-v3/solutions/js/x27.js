const fs = require("fs");

const out = JSON.parse(fs.readFileSync("in/deep.json", "utf8")).flat(Infinity);
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
