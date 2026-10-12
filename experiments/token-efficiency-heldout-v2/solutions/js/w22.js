const fs = require("fs");

const users = JSON.parse(fs.readFileSync("in/users.json", "utf8"));
const scores = JSON.parse(fs.readFileSync("in/scores.json", "utf8"));
const total = (id) => scores.filter((s) => s.user === id).reduce((a, s) => a + s.points, 0);
fs.writeFileSync("out/result.txt", users.map((u) => `${u.name} ${total(u.id)}`).join("\n") + "\n");
