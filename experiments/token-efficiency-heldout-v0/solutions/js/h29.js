const fs = require("fs");

const records = JSON.parse(fs.readFileSync("in/records.json", "utf8"));
const slim = records.map((r) => ({ id: r.id, status: r.status }));
fs.writeFileSync("out/result.json", JSON.stringify(slim, null, 2) + "\n");
