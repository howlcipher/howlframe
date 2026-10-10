#!/usr/bin/env node

function main(argv) {
  const store = Object.create(null);
  const args = argv.slice(2);
  for (let i = 0; i < args.length; i++) {
    const cmd = args[i];
    if (cmd === "set") {
      i++;
      const k = args[i];
      i++;
      const v = args[i];
      store[k] = v;
      console.log("OK");
    } else if (cmd === "get") {
      i++;
      const k = args[i];
      if (!(k in store)) console.log("NOT FOUND");
      else console.log(store[k]);
    } else if (cmd === "delete") {
      i++;
      const k = args[i];
      delete store[k];
      console.log("OK");
    } else if (cmd === "increment") {
      i++;
      const k = args[i];
      const current = k in store ? parseInt(store[k], 10) : 0;
      const next = current + 1;
      store[k] = String(next);
      console.log(next);
    } else {
      console.log("UNKNOWN CMD:", cmd);
      return 1;
    }
  }
  return 0;
}

process.exit(main(process.argv));
