const fs = require("fs");

const users = JSON.parse(fs.readFileSync("in/users.json", "utf8"));
const missing = users.filter((user) => !("email" in user)).map((user) => user.id);
fs.writeFileSync("out/result.json", JSON.stringify(missing, null, 2) + "\n");
