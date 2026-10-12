const fs = require("fs");

const orders = JSON.parse(fs.readFileSync("in/orders.json", "utf8"));
const totals = {};
for (const order of orders) {
  totals[order.cust] = (totals[order.cust] || 0) + order.total;
}
fs.writeFileSync("out/result.json", JSON.stringify(totals, null, 2) + "\n");
