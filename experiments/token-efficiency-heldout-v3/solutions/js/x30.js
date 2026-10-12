const fs = require("fs");

const key = (w) => [...w].sort().join("");
const out = fs.readFileSync("in/pairs.txt", "utf8").trim().split("\n").map((l) => {
  const [a, b] = l.split(" ");
  return key(a) === key(b) ? "yes" : "no";
});
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
