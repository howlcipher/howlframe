const fs = require("fs");

const out = [];
let cur = "";
for (const word of fs.readFileSync("in/para.txt", "utf8").trim().split(" ")) {
  if (!cur) cur = word;
  else if (cur.length + 1 + word.length <= 20) cur += " " + word;
  else {
    out.push(cur);
    cur = word;
  }
}
out.push(cur);
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
