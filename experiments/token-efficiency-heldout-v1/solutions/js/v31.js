const fs = require("fs");

const n = Number(fs.readFileSync("in/n.txt", "utf8").trim());
const out = [];
for (let i = 1; i <= n; i++) {
  out.push(i % 15 === 0 ? "FizzBuzz" : i % 3 === 0 ? "Fizz" : i % 5 === 0 ? "Buzz" : String(i));
}
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
