const fs = require("fs");

const out = JSON.parse(fs.readFileSync("in/doc.json", "utf8"));
for (const op of JSON.parse(fs.readFileSync("in/ops.json", "utf8"))) {
  const parts = op.path.split(".");
  const last = parts.pop();
  let node = out;
  for (const p of parts) node = node[p] ||= {};
  if (op.op === "set") node[last] = op.value;
  else delete node[last];
}
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
