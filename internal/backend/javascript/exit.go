package javascript

// exitJSHelper returns stderr and exit for generated JavaScript. stderr
// writes one value with no trailing newline, as on the bytecode VM. exit ends
// a Node process with the given status; a browser has no process to end, so
// there it throws and stops the current handler.
func exitJSHelper() string {
	return `function howlFrameStderr(v) {
  var s = v === null || v === undefined ? "" : String(v);
  if (typeof process !== "undefined" && process.stderr && typeof process.stderr.write === "function") {
    process.stderr.write(s);
  } else if (typeof console !== "undefined") {
    console.error(s);
  }
}
function howlFrameExit(code, where) {
  if (typeof process !== "undefined" && typeof process.exit === "function") {
    process.exit(code);
  }
  throw new Error("EXIT " + code + " at " + where);
}
`
}

// testExitJSHelper replaces howlFrameExit in app.test.js: the later function
// declaration wins, so an exit (and so a failed assert) fails the running test.
func testExitJSHelper() string {
	return `function howlFrameExit(code, where) {
  throw new Error("HowlFrame exit " + code + " at " + where);
}
`
}
