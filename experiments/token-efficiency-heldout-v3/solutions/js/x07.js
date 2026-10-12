const fs = require("fs");

const ren = JSON.parse(fs.readFileSync("in/rename.json", "utf8"));
const out = JSON.parse(fs.readFileSync("in/rows.json", "utf8")).map((r) =>
  Object.fromEntries(Object.entries(r).map(([k, v]) => [ren[k] ?? k, v])));
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
