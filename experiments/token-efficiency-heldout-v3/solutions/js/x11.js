const fs = require("fs");

const out = JSON.parse(fs.readFileSync("in/accounts.json", "utf8")).map((a) => `${a.login}: ${a.user.profile?.city ?? "unknown"}`);
fs.writeFileSync("out/result.txt", out.join("\n") + "\n");
