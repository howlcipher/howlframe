const fs = require("fs");

const lines = fs.readFileSync("in/events.log", "utf8").split("\n");
const counts = {};
for (const line of lines) {
  const level = line.split("|")[0];
  counts[level] = (counts[level] || 0) + 1;
}
fs.writeFileSync("out/result.json", JSON.stringify(counts, null, 2) + "\n");
