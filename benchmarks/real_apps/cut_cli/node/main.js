#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 2 || args[0] === "" || args[1] === "") {
    console.log("Usage: cut_cli <index> <file>");
    return 2;
  }
  const idx = parseInt(args[0], 10);
  let raw;
  try {
    raw = fs.readFileSync(args[1], "utf8");
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  for (const row of raw.split("\n")) {
    if (row === "") continue;
    const cols = row.split(",");
    if (cols.length > idx) console.log(cols[idx]);
  }
  return 0;
}

process.exit(main(process.argv));
