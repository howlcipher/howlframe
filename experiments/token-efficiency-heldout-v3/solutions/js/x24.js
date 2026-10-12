const fs = require("fs");

const gcd = (a, b) => (b ? gcd(b, a % b) : a);
const vals = fs.readFileSync("in/vals.txt", "utf8").trim().split(",").map(Number);
const out = { gcd: vals.reduce(gcd), lcm: vals.reduce((a, b) => (a / gcd(a, b)) * b) };
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
