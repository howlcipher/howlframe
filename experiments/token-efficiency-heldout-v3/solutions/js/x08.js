const fs = require("fs");

const canon = (v) =>
  Array.isArray(v) ? `[${v.map(canon).join(",")}]`
  : v !== null && typeof v === "object"
    ? `{${Object.keys(v).sort().map((k) => JSON.stringify(k) + ":" + canon(v[k])).join(",")}}`
    : JSON.stringify(v);
fs.writeFileSync("out/result.txt", canon(JSON.parse(fs.readFileSync("in/doc.json", "utf8"))) + "\n");
