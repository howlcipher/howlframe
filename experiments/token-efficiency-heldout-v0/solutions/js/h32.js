const fs = require("fs");
const path = require("path");

const entries = fs.readdirSync("in/docs", { recursive: true, withFileTypes: true });
const paths = entries
  .filter((entry) => entry.isFile() && entry.name.endsWith(".txt"))
  .map((entry) => path.relative("in/docs", path.join(entry.parentPath ?? entry.path, entry.name)).split(path.sep).join("/"))
  .sort();
fs.writeFileSync("out/result.txt", paths.join("\n") + "\n");
