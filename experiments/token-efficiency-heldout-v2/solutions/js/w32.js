const fs = require("fs");

const luhn = (digits) => {
  let total = 0;
  [...digits].reverse().forEach((c, i) => {
    const d = Number(c) * (i % 2 ? 2 : 1);
    total += d > 9 ? d - 9 : d;
  });
  return total % 10 === 0;
};
const lines = fs.readFileSync("in/ids.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.map((x) => `${x} ${luhn(x) ? "valid" : "invalid"}`).join("\n") + "\n");
