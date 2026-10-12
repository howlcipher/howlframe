const fs = require("fs");

let total = 0;
for (const line of fs.readFileSync("in/times.txt", "utf8").split("\n")) {
  const [hours, minutes] = line.split(":").map((part) => parseInt(part, 10));
  total += hours * 60 + minutes;
}
fs.writeFileSync("out/result.txt", `${total}\n`);
