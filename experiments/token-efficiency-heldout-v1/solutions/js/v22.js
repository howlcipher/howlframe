const fs = require("fs");

const word = fs.readFileSync("in/word.txt", "utf8").trim();
fs.writeFileSync("out/result.txt", (word === [...word].reverse().join("") ? "yes" : "no") + "\n");
