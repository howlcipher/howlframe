const fs = require("fs");

const values = fs.readFileSync("in/seq.txt", "utf8").split("\n").map((line) => parseInt(line, 10));
let result = "sorted";
for (let i = 1; i < values.length; i++) {
  if (values[i] < values[i - 1]) {
    result = `unsorted at ${i + 1}`;
    break;
  }
}
fs.writeFileSync("out/result.txt", result + "\n");
