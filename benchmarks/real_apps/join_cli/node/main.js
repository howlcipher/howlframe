#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 2 || args[1] === "") {
    console.log("Usage: join_cli <sep> <file>");
    return 2;
  }
  let raw;
  try {
    raw = fs.readFileSync(args[1], "utf8");
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  console.log(raw.split("\n").join(args[0]));
  return 0;
}

process.exit(main(process.argv));
