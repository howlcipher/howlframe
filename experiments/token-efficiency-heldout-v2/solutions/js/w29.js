const fs = require("fs");

const budget = Number(fs.readFileSync("in/budget.txt", "utf8"));
const spend = fs.readFileSync("in/spend.txt", "utf8").split("\n").map(Number);
let result = "within budget";
let total = 0;
for (let i = 0; i < spend.length; i++) {
  total += spend[i];
  if (total > budget) {
    result = `over at ${i + 1}`;
    break;
  }
}
fs.writeFileSync("out/result.txt", result + "\n");
