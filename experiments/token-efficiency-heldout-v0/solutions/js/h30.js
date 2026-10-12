const fs = require("fs");

const codes = JSON.parse(fs.readFileSync("in/codes.json", "utf8"));
const inverted = Object.fromEntries(Object.entries(codes).map(([key, value]) => [value, key]));
fs.writeFileSync("out/result.json", JSON.stringify(inverted, null, 2) + "\n");
