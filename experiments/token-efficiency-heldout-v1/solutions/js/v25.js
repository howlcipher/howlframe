const fs = require("fs");

const words = fs.readFileSync("in/sentence.txt", "utf8").trim().split(" ");
const longest = words.reduce((a, b) => (b.length > a.length ? b : a));
fs.writeFileSync("out/result.txt", longest + "\n");
