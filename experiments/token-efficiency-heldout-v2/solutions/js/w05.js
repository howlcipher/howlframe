const fs = require("fs");

const out = {};
for (const line of fs.readFileSync("in/ledger.csv", "utf8").split("\n").slice(1)) {
  const [, account, amount] = line.split(",");
  out[account] = (out[account] || 0) + Number(amount);
}
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
