const fs = require("fs");

const out = fs.readFileSync("in/doc.txt", "utf8").split(/\n\s*\n/).filter((p) => p.trim())
  .map((p, i) => `${i + 1}: ${p.trim().split(/\s+/).length}`);
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
