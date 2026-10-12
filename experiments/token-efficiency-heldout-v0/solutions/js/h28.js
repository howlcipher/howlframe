const fs = require("fs");

const members = JSON.parse(fs.readFileSync("in/members.json", "utf8"));
const teams = {};
for (const m of members) {
  (teams[m.team] ||= []).push(m.member);
}
fs.writeFileSync("out/result.json", JSON.stringify(teams, null, 2) + "\n");
