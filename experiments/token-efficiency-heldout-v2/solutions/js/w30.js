const fs = require("fs");

const values = { I: 1, V: 5, X: 10, L: 50, C: 100, D: 500, M: 1000 };
const out = fs.readFileSync("in/romans.txt", "utf8").split("\n").map((line) => {
  const ds = [...line].map((c) => values[c]);
  return ds.reduce((t, d, i) => (d < (ds[i + 1] || 0) ? t - d : t + d), 0);
});
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
