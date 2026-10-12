const fs = require("fs");

const out = {};
for (const s of JSON.parse(fs.readFileSync("in/sales.json", "utf8"))) {
  const r = (out[s.region] ||= { units: 0, products: [] });
  r.units += s.units;
  if (!r.products.includes(s.product)) r.products.push(s.product);
}
for (const r of Object.values(out)) r.products.sort();
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
