#!/usr/bin/env node

function main(argv) {
  const args = argv.slice(2);
  if (args.length < 2 || args[0] === "" || args[1] === "") {
    console.log("Usage: eqci_cli <a> <b>");
    return 2;
  }
  if (args[0].toLowerCase() === args[1].toLowerCase()) {
    console.log("YES");
    return 0;
  }
  console.log("NO");
  return 1;
}

process.exit(main(process.argv));
