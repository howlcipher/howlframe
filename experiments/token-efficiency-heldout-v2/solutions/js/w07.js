const fs = require("fs");

const [header, ...rows] = fs.readFileSync("in/lines.csv", "utf8").split("\n");
const out = [header + ",total"];
for (const line of rows) {
  const [, qty, unit] = line.split(",");
  out.push(`${line},${Number(qty) * Number(unit)}`);
}
fs.writeFileSync("out/result.csv", out.join("\n") + "\n");
