const fs = require("fs");

const isObj = (v) => v !== null && typeof v === "object" && !Array.isArray(v);
function flat(d, pre, out) {
  for (const [k, v] of Object.entries(d)) {
    if (isObj(v) && Object.keys(v).length) flat(v, pre + k + ".", out);
    else out[pre + k] = v;
  }
  return out;
}
const out = flat(JSON.parse(fs.readFileSync("in/nested.json", "utf8")), "", {});
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
