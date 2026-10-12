const fs = require("fs");

const [major, minor] = fs.readFileSync("in/version.txt", "utf8").trim().split(".");
fs.writeFileSync("out/result.txt", `${major}.${Number(minor) + 1}.0` + "\n");
