const fs = require("fs");

const out = [];
for (const item of fs.readFileSync("in/ranges.txt", "utf8").trim().split(",")) {
  const [a, b = a] = item.split("-").map(Number);
  for (let i = a; i <= b; i++) out.push(i);
}
fs.writeFileSync("out/result.txt", out.join(",") + "\n");
