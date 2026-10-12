const fs = require("fs");

const rows = fs.readFileSync("in/grid.csv", "utf8").trim().split("\n").map((l) => l.split(","));
const out = rows[0].map((_, j) => rows.map((r) => r[j]).join(","));
fs.writeFileSync("out/result.csv", out.join("\n") + "\n");
