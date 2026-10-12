const fs = require("fs");

const out = {};
for (const cart of JSON.parse(fs.readFileSync("in/carts.json", "utf8"))) {
  for (const item of cart.items) out[item.sku] = (out[item.sku] || 0) + item.qty;
}
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
