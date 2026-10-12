const fs = require("fs");

const lines = fs.readFileSync("in/idents.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.map((line) => line.replace(/[A-Z]/g, (c) => "_" + c.toLowerCase())).join("\n") + "\n");
