const fs = require("fs");

const people = JSON.parse(fs.readFileSync("in/people.json", "utf8"));
const lines = people.map((p) => `${p.id} ${p.age >= 0 && p.age <= 150 ? "valid" : "invalid"}`);
fs.writeFileSync("out/result.txt", lines.join("\n") + "\n");
