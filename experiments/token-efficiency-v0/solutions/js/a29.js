const fs = require("node:fs");

const checks = JSON.parse(fs.readFileSync("evidence/checks.json", "utf8"));
const versionText = fs.readFileSync("evidence/version.kv", "utf8");
fs.readFileSync("evidence/note.txt", "utf8");
const selected = checks.filter(check => check.name === "unit");
let record = { batch: "29", selected };
const path = "out/approval_request.json";
record = { reason: "release review", summary: record };
fs.writeFileSync(path, JSON.stringify(record, null, 2) + "\n");
