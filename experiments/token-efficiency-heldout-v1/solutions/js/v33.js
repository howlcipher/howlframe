const fs = require("fs");

const rows = fs.readFileSync("in/grid.txt", "utf8").split("\n");
fs.writeFileSync("out/result.json", JSON.stringify(rows.map((row) => row.split("#").length - 1), null, 2) + "\n");
