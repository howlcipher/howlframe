#!/usr/bin/env node

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 1 || args[0] === "") {
    console.log("Usage: lower_cli <text>");
    return 2;
  }
  console.log(args[0].toLowerCase());
  return 0;
}

process.exit(main(process.argv));
