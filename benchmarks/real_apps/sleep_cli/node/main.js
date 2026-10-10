#!/usr/bin/env node

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 1 || args[0] === "") {
    console.log("Usage: sleep_cli <ms>");
    return 2;
  }
  const ms = parseInt(args[0], 10);
  const start = Date.now();
  while (Date.now() - start < ms) {}
  console.log("OK");
  return 0;
}

process.exit(main(process.argv));
