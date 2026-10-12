const fs = require("fs");

const out = fs.readFileSync("in/days.txt", "utf8").trim().split("\n").map((d) => {
  const t = new Date(Date.UTC(2021, 0, Number(d)));
  return `${String(t.getUTCMonth() + 1).padStart(2, "0")}-${String(t.getUTCDate()).padStart(2, "0")}`;
});
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
