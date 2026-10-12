const fs = require("fs");

const rows = fs.readFileSync("in/stock.tsv", "utf8").split("\n").slice(1).map((line) => line.split("\t"));
rows.sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0));
fs.writeFileSync("out/result.txt", rows.map((row) => row.join("\t")).join("\n") + "\n");
