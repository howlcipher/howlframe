const fs = require("fs");

const counts = new Map();
for (const x of fs.readFileSync("in/ages.txt", "utf8").trim().split(",")) {
  const b = Math.floor(Number(x) / 10);
  counts.set(b, (counts.get(b) || 0) + 1);
}
const out = [...counts.keys()].sort((a, b) => a - b).map((b) => `${b * 10}-${b * 10 + 9}: ${counts.get(b)}`);
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
