const fs = require("fs");

const { user } = JSON.parse(fs.readFileSync("in/profile.json", "utf8"));
const result = { name: user.name, tag_count: user.tags.length };
fs.writeFileSync("out/result.json", JSON.stringify(result, null, 2) + "\n");
