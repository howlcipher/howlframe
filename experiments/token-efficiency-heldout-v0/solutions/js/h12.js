const fs = require("fs");

const text = fs.readFileSync("in/words.txt", "utf8").toLowerCase();
const counts = {};
for (const line of text.split("\n")) {
  for (const word of line.split(" ")) {
    counts[word] = (counts[word] || 0) + 1;
  }
}
fs.writeFileSync("out/result.json", JSON.stringify(counts, null, 2) + "\n");
