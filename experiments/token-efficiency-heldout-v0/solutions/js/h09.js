const fs = require("fs");

const lines = fs.readFileSync("in/data.csv", "utf8").split("\n");
const header = lines[0].split(",");
const rows = lines.slice(1).map((row) => {
  const values = row.split(",");
  return Object.fromEntries(header.map((name, i) => [name, values[i]]));
});
fs.writeFileSync("out/result.json", JSON.stringify(rows, null, 2) + "\n");
