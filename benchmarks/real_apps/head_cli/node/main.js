#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 2 || args[0] === "" || args[1] === "") {
    console.log("Usage: head_cli <n> <file>");
    return 2;
  }
  const n = parseInt(args[0], 10);
  let raw;
  try {
    raw = fs.readFileSync(args[1], "utf8");
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  let i = 0;
  for (const row of raw.split("\n")) {
    if (i < n) {
      console.log(row);
      i++;
    }
  }
  return 0;
}

process.exit(main(process.argv));
