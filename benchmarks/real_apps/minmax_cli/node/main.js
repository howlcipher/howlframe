#!/usr/bin/env node

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 2 || args[0] === "" || args[1] === "") {
    console.log("Usage: minmax_cli <a> <b>");
    return 2;
  }
  const a = parseInt(args[0], 10);
  const b = parseInt(args[1], 10);
  console.log("min=" + Math.min(a, b));
  console.log("max=" + Math.max(a, b));
  return 0;
}

process.exit(main(process.argv));
