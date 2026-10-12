const fs = require("fs");

const rows = fs.readFileSync("in/votes.txt", "utf8").trim().split("\n").map((l) => l.split(" "));
const total = rows.reduce((s, [, c]) => s + Number(c), 0);
const out = rows.map(([n, c]) => `${n} ${Math.floor((Number(c) * 100) / total)}%`);
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
