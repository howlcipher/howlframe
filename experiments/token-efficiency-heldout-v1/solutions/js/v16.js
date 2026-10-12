const fs = require("fs");

const queue = JSON.parse(fs.readFileSync("in/queue.json", "utf8"));
fs.writeFileSync("out/result.json", JSON.stringify(queue.slice(-3), null, 2) + "\n");
