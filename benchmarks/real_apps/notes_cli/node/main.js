#!/usr/bin/env node
const fs = require("fs");

function path() {
  const uri = process.env.NOTES_STORE || "file://notes.store";
  return uri.startsWith("file://") ? uri.slice(7) : uri;
}

function load() {
  try {
    return JSON.parse(fs.readFileSync(path(), "utf8"));
  } catch {
    return {};
  }
}

function save(store) {
  fs.writeFileSync(path(), JSON.stringify(store));
}

function main(argv) {
  const store = load();
  const args = argv.slice(2);
  for (let i = 0; i < args.length; i++) {
    const cmd = args[i];
    if (cmd === "add") {
      i++;
      const title = args[i];
      const n = parseInt(store.next_id || "1", 10);
      store[String(n)] = title;
      store.next_id = String(n + 1);
      console.log(`ADDED ${n} ${title}`);
    } else if (cmd === "list") {
      for (const [k, v] of Object.entries(store)) {
        if (k !== "next_id") console.log(`${k} ${v}`);
      }
    } else if (cmd === "get") {
      i++;
      const k = args[i];
      if (!(k in store)) {
        console.log("NOT FOUND");
        return 1;
      }
      console.log(`${k} ${store[k]}`);
    } else if (cmd === "delete") {
      i++;
      const k = args[i];
      if (!(k in store)) {
        console.log("NOT FOUND");
        return 1;
      }
      delete store[k];
      console.log(`DELETED ${k}`);
    } else {
      console.log("UNKNOWN");
      return 1;
    }
  }
  save(store);
  return 0;
}

process.exit(main(process.argv));
