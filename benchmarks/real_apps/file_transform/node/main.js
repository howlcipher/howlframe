#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 2 || args[0] === "" || args[1] === "") {
    console.log("Usage: file_transform <in.csv> <out.jsonl>");
    return 2;
  }
  let raw;
  try {
    raw = fs.readFileSync(args[0], "utf8");
  } catch (e) {
    console.log("error=" + e);
    return 2;
  }
  let n = 0;
  let buf = "";
  for (const row of raw.split("\n")) {
    if (row === "") continue;
    const cols = row.split(",");
    if (cols.length >= 2) {
      n++;
      buf += JSON.stringify({ c0: cols[0], c1: cols[1] }) + "\n";
    }
  }
  fs.writeFileSync(args[1], buf);
  console.log(`rows=${n}`);
  console.log(`out=${args[1]}`);
  return 0;
}

process.exit(main(process.argv));
