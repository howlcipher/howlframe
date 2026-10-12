const fs = require("fs");

const entries = fs.readdirSync("in/docs", { recursive: true, withFileTypes: true });
const count = entries.filter((entry) => entry.isFile()).length;
fs.writeFileSync("out/result.txt", `${count}\n`);
