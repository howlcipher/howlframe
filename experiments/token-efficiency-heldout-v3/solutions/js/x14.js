const fs = require("fs");

const out = {};
for (const [k, v] of Object.entries(JSON.parse(fs.readFileSync("in/prices.json", "utf8")))) {
  out[k] = `$${Math.floor(v / 100)}.${String(v % 100).padStart(2, "0")}`;
}
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
