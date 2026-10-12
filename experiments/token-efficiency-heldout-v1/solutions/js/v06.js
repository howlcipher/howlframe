const fs = require("fs");

const rows = fs.readFileSync("in/stock.tsv", "utf8").split("\n").slice(1).map((line) => line.split("\t"));
fs.writeFileSync("out/result.txt", rows.filter((row) => Number(row[1]) === 0).map((row) => row[0]).join("\n") + "\n");
