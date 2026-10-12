const fs = require("fs");

const orders = JSON.parse(fs.readFileSync("in/orders.json", "utf8"));
fs.writeFileSync("out/result.json", JSON.stringify({ count: orders.length, total: orders.reduce((sum, order) => sum + order.total, 0) }, null, 2) + "\n");
