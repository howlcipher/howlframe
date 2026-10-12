const fs = require("fs");

const out = {};
for (const [k, v] of Object.entries(JSON.parse(fs.readFileSync("in/flat.json", "utf8")))) {
  const parts = k.split(".");
  const last = parts.pop();
  let node = out;
  for (const p of parts) node = node[p] ||= {};
  node[last] = v;
}
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
