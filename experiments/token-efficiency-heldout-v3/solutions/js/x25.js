const fs = require("fs");

const out = fs.readFileSync("in/nums.txt", "utf8").trim().split("\n").map((n) => {
  let d = n;
  while (d.length > 1) d = String([...d].reduce((s, c) => s + Number(c), 0));
  return `${n} -> ${d}`;
});
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
