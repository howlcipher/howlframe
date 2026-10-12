const fs = require("fs");

const out = {};
for (const s of JSON.parse(fs.readFileSync("in/staff.json", "utf8"))) {
  if (s.manager) (out[s.manager] ||= []).push(s.name);
}
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
