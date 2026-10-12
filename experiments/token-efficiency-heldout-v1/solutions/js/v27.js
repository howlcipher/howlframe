const fs = require("fs");

const orders = JSON.parse(fs.readFileSync("in/orders.json", "utf8"));
fs.writeFileSync("out/result.txt", orders.filter((order) => order.total >= 100).map((order) => order.id).join("\n") + "\n");
