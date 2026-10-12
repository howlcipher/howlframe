const fs = require("fs");

const lines = fs.readFileSync("in/years.txt", "utf8").split("\n");
const leap = (y) => y % 4 === 0 && (y % 100 !== 0 || y % 400 === 0);
fs.writeFileSync("out/result.txt", lines.map((y) => `${y} ${leap(Number(y)) ? "leap" : "common"}`).join("\n") + "\n");
