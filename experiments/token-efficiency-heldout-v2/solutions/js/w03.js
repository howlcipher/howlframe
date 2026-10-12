const fs = require("fs");

const lines = fs.readFileSync("in/settings.txt", "utf8").split("\n").map((line) => line.split("#")[0].replace(/ +$/, ""));
fs.writeFileSync("out/result.txt", lines.filter((line) => line).join("\n") + "\n");
