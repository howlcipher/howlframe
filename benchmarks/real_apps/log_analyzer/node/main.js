#!/usr/bin/env node
const fs = require("fs");

function main(argv) {
  if (argv.length < 3 || argv[2] === "") {
    console.log("HowlFrame Log Analyzer");
    console.log("Usage: log_analyzer <log-file>");
    return 2;
  }
  const logFile = argv[2];
  let content;
  try {
    content = fs.readFileSync(logFile, "utf8");
  } catch (err) {
    console.log("HowlFrame Log Analyzer");
    console.log("file=", logFile);
    console.log("error=", err.message || err);
    return 2;
  }
  const lines = content.split("\n");
  let errors = 0, warnings = 0, todos = 0;
  for (const line of lines) {
    if (/ERROR|FATAL/.test(line)) errors++;
    else if (/WARN|WARNING/.test(line)) warnings++;
    else if (/TODO|FIXME/.test(line)) todos++;
  }
  console.log("HowlFrame Log Analyzer");
  console.log("file=", logFile);
  console.log("lines=", lines.length);
  console.log("errors=", errors);
  console.log("warnings=", warnings);
  console.log("todo=", todos);
  if (errors > 0) {
    console.log("status=ATTENTION");
    return 1;
  }
  console.log("status=CLEAN");
  return 0;
}

process.exit(main(process.argv));
