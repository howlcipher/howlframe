const fs = require("fs");

const [head, ...lines] = fs.readFileSync("in/sites.csv", "utf8").trimEnd().split("\n");
const out = [head];
let prev = null;
for (const line of lines) {
  prev = line.split(",").map((c, j) => c || prev[j]);
  out.push(prev.join(","));
}
fs.writeFileSync("out/result.csv", out.join("\n") + "\n");
