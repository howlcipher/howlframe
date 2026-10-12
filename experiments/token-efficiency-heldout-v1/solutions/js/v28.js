const fs = require("fs");

const orders = JSON.parse(fs.readFileSync("in/orders.json", "utf8"));
fs.writeFileSync("out/result.json", JSON.stringify([...new Set(orders.map((order) => order.cust))], null, 2) + "\n");
