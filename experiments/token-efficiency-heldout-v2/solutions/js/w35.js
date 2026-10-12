const fs = require("fs");

const prices = JSON.parse(fs.readFileSync("in/prices.json", "utf8"));
const target = Number(fs.readFileSync("in/target.txt", "utf8"));
let result = "none";
search: for (let j = 1; j < prices.length; j++) {
  for (let i = 0; i < j; i++) {
    if (prices[i] + prices[j] === target) {
      result = `${i} ${j}`;
      break search;
    }
  }
}
fs.writeFileSync("out/result.txt", result + "\n");
