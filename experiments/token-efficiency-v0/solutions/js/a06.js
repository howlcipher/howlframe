const fs = require("node:fs");

const checks = JSON.parse(fs.readFileSync("evidence/checks.json", "utf8"));
const versionText = fs.readFileSync("evidence/version.kv", "utf8");
fs.readFileSync("evidence/note.txt", "utf8");
const selected = checks.filter(check => check.status === "PASS");
let record = { batch: "06", selected };
const path = "out/release.json";
fs.writeFileSync(path, JSON.stringify(record, null, 2) + "\n");
