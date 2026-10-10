#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 2 || args[0] === "" || args[1] === "") {
    console.log("Usage: getd_cli <file> <key> [default]");
    return 2;
  }
  const def = args.length > 2 && args[2] !== "" ? args[2] : "MISSING";
  let obj;
  try {
    obj = JSON.parse(fs.readFileSync(args[0], "utf8"));
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  if (Object.prototype.hasOwnProperty.call(obj, args[1])) {
    console.log(obj[args[1]]);
    return 0;
  }
  console.log(def);
  return 0;
}

process.exit(main(process.argv));
