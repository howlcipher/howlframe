const fs = require("fs");

const items = JSON.parse(fs.readFileSync("in/items.json", "utf8"));
const byId = Object.fromEntries(items.map((item) => [item.id, item.name]));
fs.writeFileSync("out/result.json", JSON.stringify(byId, null, 2) + "\n");
