const fs = require("fs");

const n = Number(fs.readFileSync("in/n.txt", "utf8"));
const primes = [];
for (let p = 2; p <= n; p++) {
  let prime = true;
  for (let d = 2; d * d <= p; d++) if (p % d === 0) prime = false;
  if (prime) primes.push(p);
}
fs.writeFileSync("out/result.txt", primes.join(" ") + "\n");
