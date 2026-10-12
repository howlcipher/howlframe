const fs = require("node:fs");

const checks = JSON.parse(fs.readFileSync("evidence/checks.json", "utf8"));
const versionText = fs.readFileSync("evidence/version.kv", "utf8");
fs.readFileSync("evidence/note.txt", "utf8");
const selected = checks.filter(check => check.name === "unit");
let record = { batch: "11", selected };
const versions = Object.fromEntries(versionText.trim().split("\n").map(line => line.split("=")));
const current = versions.current.split(".").map(Number);
const proposed = versions.proposed.split(".").map(Number);
let newer = false;
for (let i = 0; i < 3; i++) {
    if (proposed[i] !== current[i]) {
        newer = proposed[i] > current[i];
        break;
    }
}
let path;
if (checks.every(check => check.status === "PASS") && newer) {
    path = "out/release.json";
} else {
    path = "out/approval_request.json";
    record = { reason: "blocked", summary: {} };
}
fs.writeFileSync(path, JSON.stringify(record, null, 2) + "\n");
