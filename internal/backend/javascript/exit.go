package javascript

// exitJSHelper returns stderr and exit for generated JavaScript. stderr
// writes one value with no trailing newline, as on the bytecode VM.
//
// In app.js, exit ends a Node process with the given status; a browser has no
// process to end, so there it throws and stops the current handler. In
// app.test.js (forTests) exit always throws, so a failed assert fails the
// running node:test test and the remaining tests still run. The thrown
// error is marked howlFrameExit so no try_let catches it: on the VM, exit
// ends the program whatever handler encloses it (bugs.md #67). Each file gets
// exactly one declaration, which keeps it valid in strict and module code.
func exitJSHelper(forTests bool) string {
	exit := `function howlFrameExit(code, where) {
  if (typeof process !== "undefined" && typeof process.exit === "function") {
    process.exit(code);
  }
  var e = new Error("EXIT " + code + " at " + where);
  e.howlFrameExit = true;
  throw e;
}
`
	if forTests {
		exit = `function howlFrameExit(code, where) {
  var e = new Error("HowlFrame exit " + code + " at " + where);
  e.howlFrameExit = true;
  throw e;
}
`
	}
	return `function howlFrameStderr(v) {
  var s = v === null || v === undefined ? "" : String(v);
  if (typeof process !== "undefined" && process.stderr && typeof process.stderr.write === "function") {
    process.stderr.write(s);
  } else if (typeof console !== "undefined") {
    console.error(s);
  }
}
` + exit
}
