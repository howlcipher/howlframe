const fs = require("fs");

const isObj = (v) => v !== null && typeof v === "object" && !Array.isArray(v);
const cfg = JSON.parse(fs.readFileSync("in/config.json", "utf8"));
const out = fs.readFileSync("in/paths.txt", "utf8").trim().split("\n").map((line) => {
  let v = cfg;
  for (const k of line.split(".")) {
    if (!isObj(v) || !(k in v)) return "missing";
    v = v[k];
  }
  return JSON.stringify(v);
});
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
