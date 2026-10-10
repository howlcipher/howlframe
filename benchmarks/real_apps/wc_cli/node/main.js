#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 1 || args[0] === "") {
    console.log("Usage: wc_cli <file>");
    return 2;
  }
  let raw;
  try {
    raw = fs.readFileSync(args[0]);
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  const rows = raw.toString("utf8").split("\n");
  let wc = 0;
  for (const row of rows) {
    for (const w of row.split(" ")) {
      if (w !== "") wc++;
    }
  }
  console.log(`lines=${rows.length}`);
  console.log(`words=${wc}`);
  console.log(`bytes=${raw.length}`);
  return 0;
}

process.exit(main(process.argv));
