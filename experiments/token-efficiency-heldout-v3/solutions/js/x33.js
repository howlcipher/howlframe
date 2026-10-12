const fs = require("fs");

const spans = fs.readFileSync("in/spans.txt", "utf8").trim().split("\n")
  .map((l) => l.split("-").map(Number)).sort((x, y) => x[0] - y[0]);
const merged = [];
for (const [a, b] of spans) {
  const last = merged[merged.length - 1];
  if (last && a <= last[1]) last[1] = Math.max(last[1], b);
  else merged.push([a, b]);
}
const out = merged.map(([a, b]) => `${a}-${b}`);
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
