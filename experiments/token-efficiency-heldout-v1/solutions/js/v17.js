const fs = require("fs");

const queue = JSON.parse(fs.readFileSync("in/queue.json", "utf8"));
fs.writeFileSync("out/result.txt", String(queue.indexOf("build")) + "\n");
