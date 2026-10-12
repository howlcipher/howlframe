const fs = require("fs");

const title = fs.readFileSync("in/title.txt", "utf8").trim().toLowerCase();
fs.writeFileSync("out/result.txt", title.split(" ").filter(Boolean).join("-") + "\n");
