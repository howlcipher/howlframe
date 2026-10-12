const fs = require("fs");

const out = [];
let cur = "";
for (const line of fs.readFileSync("in/script.txt", "utf8").trimEnd().split("\n")) {
  if (line.endsWith("\\")) cur += line.slice(0, -1);
  else { out.push(cur + line); cur = ""; }
}
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
