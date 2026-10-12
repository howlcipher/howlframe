const fs = require("fs");

const out = JSON.parse(fs.readFileSync("in/tickets.json", "utf8")).filter((t) => (t.meta.tags || []).includes("urgent")).map((t) => t.id);
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
