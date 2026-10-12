const fs = require("fs");

const rows = (p) => fs.readFileSync(p, "utf8").trim().split("\n").slice(1);
const dept = Object.fromEntries(rows("in/dept.csv").map((l) => l.split(",")));
const out = ["id,name,dept", ...rows("in/emp.csv").map((l) => `${l},${dept[l.split(",")[0]] ?? ""}`)];
fs.writeFileSync("out/result.csv", out.join("\n") + "\n");
