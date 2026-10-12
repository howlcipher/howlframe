const fs = require("fs");

const rows = fs.readFileSync("in/stock.tsv", "utf8").split("\n").slice(1).map((line) => line.split("\t"));
const total = rows.reduce((sum, row) => sum + Number(row[1]) * Number(row[2]), 0);
fs.writeFileSync("out/result.txt", String(total) + "\n");
