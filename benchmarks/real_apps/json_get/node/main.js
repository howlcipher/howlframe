#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 2 || args[0] === "" || args[1] === "") {
    console.log("Usage: json_get <file> <key>");
    return 2;
  }
  let obj;
  try {
    obj = JSON.parse(fs.readFileSync(args[0], "utf8"));
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  if (!(args[1] in obj)) {
    console.log("NOT FOUND");
    return 1;
  }
  console.log(obj[args[1]]);
  return 0;
}

process.exit(main(process.argv));
