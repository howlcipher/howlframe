const fs = require("fs");

const lines = fs.readFileSync("in/data.csv", "utf8").split("\n");
const col = lines[0].split(",").indexOf("dept");
const counts = {};
for (const row of lines.slice(1)) {
  const dept = row.split(",")[col];
  counts[dept] = (counts[dept] || 0) + 1;
}
fs.writeFileSync("out/result.json", JSON.stringify(counts, null, 2) + "\n");
