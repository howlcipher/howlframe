const fs = require("fs");

const pad = (n) => String(n).padStart(2, "0");
const out = fs.readFileSync("in/seconds.txt", "utf8").split("\n").map((x) => {
  const s = Number(x);
  return `${Math.floor(s / 3600)}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`;
});
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
