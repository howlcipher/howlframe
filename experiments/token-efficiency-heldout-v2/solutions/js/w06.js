const fs = require("fs");

const out = {};
for (const line of fs.readFileSync("in/ledger.csv", "utf8").split("\n").slice(1)) {
  const [date, , amount] = line.split(",");
  const month = date.slice(0, 7);
  out[month] = (out[month] || 0) + Number(amount);
}
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
