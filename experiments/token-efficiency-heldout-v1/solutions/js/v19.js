const fs = require("fs");

const queue = JSON.parse(fs.readFileSync("in/queue.json", "utf8"));
const counts = {};
for (const item of queue) {
  counts[item] = (counts[item] || 0) + 1;
}
fs.writeFileSync("out/result.json", JSON.stringify(counts, null, 2) + "\n");
