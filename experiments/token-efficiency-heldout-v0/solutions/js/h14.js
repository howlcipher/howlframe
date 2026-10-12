const fs = require("fs");

const lines = fs.readFileSync("in/app.log", "utf8").split("\n");
const count = lines.filter((line) => line.includes("ERROR")).length;
fs.writeFileSync("out/result.txt", `${count}\n`);
