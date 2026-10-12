const fs = require("fs");

const old = JSON.parse(fs.readFileSync("in/old.json", "utf8"));
const nw = JSON.parse(fs.readFileSync("in/new.json", "utf8"));
const out = {
  added: Object.keys(nw).filter((k) => !(k in old)).sort(),
  removed: Object.keys(old).filter((k) => !(k in nw)).sort(),
  changed: Object.keys(old).filter((k) => k in nw && JSON.stringify(old[k]) !== JSON.stringify(nw[k])).sort(),
};
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
