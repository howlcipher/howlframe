const fs = require("fs");

const depth = (v) =>
  v !== null && typeof v === "object" ? 1 + Math.max(0, ...Object.values(v).map(depth)) : 0;
fs.writeFileSync("out/result.txt", depth(JSON.parse(fs.readFileSync("in/tree.json", "utf8"))) + "\n");
