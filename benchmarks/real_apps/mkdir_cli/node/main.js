#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 1 || args[0] === "") {
    console.log("Usage: mkdir_cli <path>");
    return 2;
  }
  fs.mkdirSync(args[0], { recursive: true });
  console.log("OK");
  return 0;
}

process.exit(main(process.argv));
