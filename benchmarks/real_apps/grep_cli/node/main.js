#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 2 || args[0] === "" || args[1] === "") {
    console.log("Usage: grep_cli <pattern> <file>");
    return 2;
  }
  const [pat, path] = args;
  let raw;
  try {
    raw = fs.readFileSync(path, "utf8");
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  const rows = raw.split(/\r?\n/);
  if (rows.length && rows[rows.length - 1] === "") rows.pop();
  const re = new RegExp(pat);
  let n = 0;
  for (const row of rows) {
    if (re.test(row)) {
      n++;
      console.log(row);
    }
  }
  console.log(`hits=${n}`);
  return n > 0 ? 0 : 1;
}

process.exit(main(process.argv));
