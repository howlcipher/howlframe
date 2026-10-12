const fs = require("fs");

const text = fs.readFileSync("in/text.txt", "utf8");
fs.writeFileSync("out/result.txt", text.replaceAll("foo", "bar") + "\n");
