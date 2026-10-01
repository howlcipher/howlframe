package javascript

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestJSNumericContractSemantics(t *testing.T) {
	script := numericJSHelper() + `
const assert = require("node:assert");

// to_int semantics
assert.strictEqual(howlFrameToInt(42), 42);
assert.strictEqual(howlFrameToInt(3.9), 3);
assert.strictEqual(howlFrameToInt(-3.9), -3);
assert.strictEqual(howlFrameToInt(0.0), 0);
assert.strictEqual(howlFrameToInt("42"), 42);
assert.strictEqual(howlFrameToInt(" -7 "), -7);
assert.throws(() => howlFrameToInt(""), /CONVERSION_ERROR/);
assert.throws(() => howlFrameToInt("abc"), /CONVERSION_ERROR/);
assert.throws(() => howlFrameToInt(NaN), /CONVERSION_ERROR/);
assert.throws(() => howlFrameToInt(Infinity), /CONVERSION_ERROR/);

// to_float semantics
assert.strictEqual(howlFrameToFloat(42), 42);
assert.strictEqual(howlFrameToFloat(2.5), 2.5);
assert.strictEqual(howlFrameToFloat("3.14"), 3.14);
assert.throws(() => howlFrameToFloat(""), /CONVERSION_ERROR/);
assert.throws(() => howlFrameToFloat("   "), /CONVERSION_ERROR/);
assert.throws(() => howlFrameToFloat("abc"), /CONVERSION_ERROR/);
assert.throws(() => howlFrameToFloat("Infinity"), /CONVERSION_ERROR/);
assert.throws(() => howlFrameToFloat("NaN"), /CONVERSION_ERROR/);

// real division semantics
assert.strictEqual(howlFrameDiv(9, 2), 4.5);
assert.strictEqual(howlFrameDiv(8, 2), 4.0);
assert.strictEqual(howlFrameDiv(9.0, 2.0), 4.5);
assert.throws(() => howlFrameDiv(1, 0), /RUNTIME_ERROR: division by zero/);
assert.throws(() => howlFrameDiv(0, 0), /RUNTIME_ERROR: division by zero/);
`
	dir := t.TempDir()
	path := filepath.Join(dir, "numeric_test.js")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil {
		t.Fatalf("node numeric contract assertions failed: %v\n%s", err, out)
	}
}
