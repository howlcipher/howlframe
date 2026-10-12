const fs = require("fs");

const rows = fs.readFileSync("in/stock.tsv", "utf8").split("\n").slice(1).map((line) => line.split("\t"));
const stock = Object.fromEntries(rows.map((row) => [row[0], Number(row[1])]));
fs.writeFileSync("out/result.json", JSON.stringify(stock, null, 2) + "\n");
