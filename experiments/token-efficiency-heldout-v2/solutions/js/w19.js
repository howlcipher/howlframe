const fs = require("fs");

const msg = fs.readFileSync("in/msg.txt", "utf8").trim();
const out = msg.replace(/[a-z]/g, (c) => String.fromCharCode(((c.charCodeAt(0) - 97 + 3) % 26) + 97));
fs.writeFileSync("out/result.txt", out + "\n");
