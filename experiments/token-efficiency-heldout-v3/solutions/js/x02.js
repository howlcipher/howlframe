const fs = require("fs");

const isObj = (v) => v !== null && typeof v === "object" && !Array.isArray(v);
function merge(a, b) {
  for (const [k, v] of Object.entries(b)) a[k] = isObj(a[k]) && isObj(v) ? merge(a[k], v) : v;
  return a;
}
const out = merge(JSON.parse(fs.readFileSync("in/base.json", "utf8")), JSON.parse(fs.readFileSync("in/override.json", "utf8")));
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
