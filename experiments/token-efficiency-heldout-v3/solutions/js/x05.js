const fs = require("fs");

const out = { present: [], null: [], absent: [] };
for (const c of JSON.parse(fs.readFileSync("in/contacts.json", "utf8"))) {
  out[!("email" in c) ? "absent" : c.email === null ? "null" : "present"].push(c.id);
}
fs.writeFileSync("out/result.json", JSON.stringify(out, null, 2) + "\n");
