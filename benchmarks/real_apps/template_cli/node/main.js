#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 3 || args[0] === "" || args[1] === "" || args[2] === "") {
    console.log("Usage: template_cli <file> <key> <value>");
    return 2;
  }
  let s;
  try {
    s = fs.readFileSync(args[0], "utf8");
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  const needle = "{{" + args[1] + "}}";
  const parts = s.split(needle);
  if (parts.length < 2) {
    console.log("NO PLACEHOLDER");
    return 1;
  }
  process.stdout.write(parts[0] + args[2] + parts[1]);
  return 0;
}

process.exit(main(process.argv));
