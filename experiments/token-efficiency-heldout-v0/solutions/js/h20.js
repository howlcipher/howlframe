const fs = require("fs");

const tags = fs.readFileSync("in/tags.txt", "utf8").split(";");
const unique = [...new Set(tags)];
fs.writeFileSync("out/result.json", JSON.stringify(unique, null, 2) + "\n");
