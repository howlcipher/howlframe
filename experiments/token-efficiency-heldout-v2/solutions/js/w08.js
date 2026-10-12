const fs = require("fs");

const rows = fs.readFileSync("in/team.csv", "utf8").split("\n").slice(1).map((line) => line.split(","));
const width = Math.max(...rows.map(([name]) => name.length));
fs.writeFileSync("out/result.txt", rows.map(([name, role]) => `${name.padEnd(width)} | ${role}`).join("\n") + "\n");
