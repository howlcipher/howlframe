const fs = require("fs");

const numbers = fs.readFileSync("in/numbers.txt", "utf8").split("\n").map((line) => parseInt(line, 10));
const result = {
  min: Math.min(...numbers),
  max: Math.max(...numbers),
  sum: numbers.reduce((a, b) => a + b, 0),
};
fs.writeFileSync("out/result.json", JSON.stringify(result, null, 2) + "\n");
