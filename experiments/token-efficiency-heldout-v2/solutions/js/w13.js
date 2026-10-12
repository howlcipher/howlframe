const fs = require("fs");

const jobs = JSON.parse(fs.readFileSync("in/jobs.json", "utf8"));
const out = [];
for (let i = 0; i < jobs.length; i += 3) out.push(jobs.slice(i, i + 3));
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
