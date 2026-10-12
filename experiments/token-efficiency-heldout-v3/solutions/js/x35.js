const fs = require("fs");

const deps = JSON.parse(fs.readFileSync("in/deps.json", "utf8"));
const out = [];
while (out.length < Object.keys(deps).length) {
  out.push(Object.keys(deps).filter((t) => !out.includes(t) && deps[t].every((d) => out.includes(d))).sort()[0]);
}
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
