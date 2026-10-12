const fs = require("fs");

const ring = JSON.parse(fs.readFileSync("in/ring.json", "utf8"));
const k = Number(fs.readFileSync("in/k.txt", "utf8")) % ring.length;
fs.writeFileSync("out/result.json", JSON.stringify([...ring.slice(k), ...ring.slice(0, k)], null, 2) + "\n");
