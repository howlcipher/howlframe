const fs = require("fs");

const env = JSON.parse(fs.readFileSync("in/env.json", "utf8"));
const out = Object.keys(env).sort().map((k) => ({ key: k, value: env[k] }));
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
