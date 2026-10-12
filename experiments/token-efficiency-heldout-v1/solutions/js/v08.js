const fs = require("fs");

const rows = fs.readFileSync("in/stock.tsv", "utf8").split("\n").slice(1).map((line) => line.split("\t"));
const best = rows.reduce((a, b) => (Number(b[2]) > Number(a[2]) ? b : a));
fs.writeFileSync("out/result.txt", best[0] + "\n");
