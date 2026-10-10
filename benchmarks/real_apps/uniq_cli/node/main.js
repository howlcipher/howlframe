#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 1 || args[0] === "") {
    console.log("Usage: uniq_cli <file>");
    return 2;
  }
  let raw;
  try {
    raw = fs.readFileSync(args[0], "utf8");
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  const seen = new Set();
  for (const row of raw.split("\n")) {
    if (row !== "" && !seen.has(row)) {
      seen.add(row);
      console.log(row);
    }
  }
  return 0;
}

process.exit(main(process.argv));
