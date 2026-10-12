const fs = require("fs");

const ids = JSON.parse(fs.readFileSync("in/ids.json", "utf8"));
let result = "ok";
const seen = new Set();
for (const id of ids) {
  if (seen.has(id)) {
    result = `duplicate:${id}`;
    break;
  }
  seen.add(id);
}
fs.writeFileSync("out/result.txt", result + "\n");
