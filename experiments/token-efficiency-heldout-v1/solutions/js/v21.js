const fs = require("fs");

const path = fs.readFileSync("in/path.txt", "utf8").trim();
fs.writeFileSync("out/result.txt", path.slice(path.lastIndexOf(".") + 1) + "\n");
