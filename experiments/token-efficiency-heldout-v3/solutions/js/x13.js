const fs = require("fs");

const out = {};
for (const line of fs.readFileSync("in/requests.jsonl", "utf8").trim().split("\n")) {
  const r = JSON.parse(line);
  out[r.level] = Math.max(out[r.level] ?? r.ms, r.ms);
}
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
