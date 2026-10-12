const fs = require("fs");

const orders = JSON.parse(fs.readFileSync("in/orders.json", "utf8"));
const cheapest = orders.reduce((a, b) => (b.total < a.total ? b : a));
fs.writeFileSync("out/result.txt", cheapest.id + "\n");
