#!/usr/bin/env node

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 1 || args[0] === "") {
    console.log("Usage: env_cli <name>");
    return 2;
  }
  const v = process.env[args[0]] || "";
  if (v === "") {
    console.log("MISSING");
    return 1;
  }
  console.log(v);
  return 0;
}

process.exit(main(process.argv));
