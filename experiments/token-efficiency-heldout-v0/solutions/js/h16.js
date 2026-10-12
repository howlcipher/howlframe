const fs = require("fs");

const lines = fs.readFileSync("in/config.kv", "utf8").split("\n");
const config = {};
for (const line of lines) {
  const i = line.indexOf("=");
  config[line.slice(0, i)] = line.slice(i + 1);
}
fs.writeFileSync("out/result.json", JSON.stringify(config, null, 2) + "\n");
