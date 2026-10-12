const fs = require("fs");

const check = (p) => (p.length < 8 ? "too short" : !/[0-9]/.test(p) ? "no digit" : !/[A-Z]/.test(p) ? "no upper" : "ok");
const lines = fs.readFileSync("in/passwords.txt", "utf8").split("\n");
fs.writeFileSync("out/result.txt", lines.map(check).join("\n") + "\n");
