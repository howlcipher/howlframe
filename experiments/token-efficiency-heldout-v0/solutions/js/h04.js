const fs = require("fs");

const lines = fs.readFileSync("in/text.txt", "utf8").split("\n");
const kept = lines.map((line) => line.trim()).filter((line) => line !== "");
fs.writeFileSync("out/result.txt", kept.join("\n") + "\n");
