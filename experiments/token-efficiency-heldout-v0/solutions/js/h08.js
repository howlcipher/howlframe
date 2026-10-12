const fs = require("fs");

const lines = fs.readFileSync("in/data.csv", "utf8").split("\n");
const col = lines[0].split(",").indexOf("amount");
const total = lines.slice(1).reduce((sum, row) => sum + parseInt(row.split(",")[col], 10), 0);
fs.writeFileSync("out/result.txt", `${total}\n`);
