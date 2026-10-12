const fs = require("fs");

const out = [];
let inside = false;
for (const line of fs.readFileSync("in/app.ini", "utf8").split("\n")) {
  if (line.startsWith("[")) inside = line === "[net]";
  else if (inside && line) out.push(line);
}
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
