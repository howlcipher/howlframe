const fs = require("fs");

const stock = JSON.parse(fs.readFileSync("in/stock.json", "utf8"));
for (const m of JSON.parse(fs.readFileSync("in/moves.json", "utf8"))) stock[m.sku] = (stock[m.sku] || 0) + m.delta;
fs.writeFileSync("out/result.json", JSON.stringify(stock, null, 2) + "\n");
