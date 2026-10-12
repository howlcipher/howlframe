const fs = require("fs");

const schema = JSON.parse(fs.readFileSync("in/schema.json", "utf8"));
const config = JSON.parse(fs.readFileSync("in/config.json", "utf8"));
const missing = schema.filter((key) => !(key in config));
fs.writeFileSync("out/result.json", JSON.stringify(missing, null, 2) + "\n");
