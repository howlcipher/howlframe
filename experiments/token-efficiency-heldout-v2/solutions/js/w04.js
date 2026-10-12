const fs = require("fs");

const lines = fs.readFileSync("in/headings.txt", "utf8").split("\n");
const title = (line) => line.split(" ").map((w) => w.slice(0, 1).toUpperCase() + w.slice(1)).join(" ");
fs.writeFileSync("out/result.txt", lines.map(title).join("\n") + "\n");
