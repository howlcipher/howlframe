const fs = require("fs");

const emails = fs.readFileSync("in/emails.txt", "utf8").split("\n");
const domains = emails.map((email) => email.split("@")[1]);
fs.writeFileSync("out/result.txt", domains.join("\n") + "\n");
