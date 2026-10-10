#!/usr/bin/env node

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 1 || args[0] === "") {
    console.log("Usage: upper_cli <text>");
    return 2;
  }
  console.log(args[0].toUpperCase());
  return 0;
}

process.exit(main(process.argv));
