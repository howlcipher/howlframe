const fs = require("fs");

const lines = fs.readFileSync("in/data.csv", "utf8").split("\n");
const col = lines[0].split(",").indexOf("email");
const emails = lines.slice(1).map((row) => row.split(",")[col]);
fs.writeFileSync("out/result.txt", emails.join("\n") + "\n");
