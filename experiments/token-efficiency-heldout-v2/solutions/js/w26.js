const fs = require("fs");

let n = Number(fs.readFileSync("in/n.txt", "utf8"));
let steps = 0;
while (n !== 1) {
  n = n % 2 === 0 ? n / 2 : 3 * n + 1;
  steps++;
}
fs.writeFileSync("out/result.txt", steps + "\n");
