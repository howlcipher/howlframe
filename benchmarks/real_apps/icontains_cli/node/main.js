#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 2 || args[0] === "" || args[1] === "") {
    console.log("Usage: icontains_cli <needle> <file>");
    return 2;
  }
  let s;
  try {
    s = fs.readFileSync(args[1], "utf8");
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  if (s.toLowerCase().includes(args[0].toLowerCase())) {
    console.log("FOUND");
    return 0;
  }
  console.log("MISSING");
  return 1;
}

process.exit(main(process.argv));
