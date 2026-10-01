package gogen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoNumericContractSemantics(t *testing.T) {
	mainGo := `package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

` + numericHelperSource() + `

func assertEqual[T comparable](got, want T, label string) {
	if got != want {
		panic(fmt.Sprintf("%s: got %v, want %v", label, got, want))
	}
}

func assertPanics(f func(), label string) {
	defer func() {
		if r := recover(); r == nil {
			panic(fmt.Sprintf("%s: expected panic, but did not panic", label))
		}
	}()
	f()
}

func main() {
	// to_int semantics
	assertEqual(howlFrameToInt(42), 42, "to_int(42)")
	assertEqual(howlFrameToInt(int64(42)), 42, "to_int(int64(42))")
	assertEqual(howlFrameToInt(3.9), 3, "to_int(3.9) truncates toward zero")
	assertEqual(howlFrameToInt(-3.9), -3, "to_int(-3.9) truncates toward zero")
	assertEqual(howlFrameToInt(0.0), 0, "to_int(0.0)")
	assertEqual(howlFrameToInt("42"), 42, "to_int(\"42\")")
	assertEqual(howlFrameToInt(" -7 "), -7, "to_int(\" -7 \")")
	assertPanics(func() { howlFrameToInt("") }, "to_int(\"\")")
	assertPanics(func() { howlFrameToInt("abc") }, "to_int(\"abc\")")
	assertPanics(func() { howlFrameToInt(math.NaN()) }, "to_int(NaN)")
	assertPanics(func() { howlFrameToInt(math.Inf(1)) }, "to_int(Inf)")

	// to_float semantics
	assertEqual(howlFrameToFloat(42), 42.0, "to_float(42)")
	assertEqual(howlFrameToFloat(int64(42)), 42.0, "to_float(int64(42))")
	assertEqual(howlFrameToFloat(2.5), 2.5, "to_float(2.5)")
	assertEqual(howlFrameToFloat("3.14"), 3.14, "to_float(\"3.14\")")
	assertPanics(func() { howlFrameToFloat("") }, "to_float(\"\")")
	assertPanics(func() { howlFrameToFloat("   ") }, "to_float(\"   \")")
	assertPanics(func() { howlFrameToFloat("abc") }, "to_float(\"abc\")")
	assertPanics(func() { howlFrameToFloat("NaN") }, "to_float(\"NaN\")")
	assertPanics(func() { howlFrameToFloat("Inf") }, "to_float(\"Inf\")")

	// real division semantics
	assertEqual(howlFrameDiv(9, 2), 4.5, "9 / 2")
	assertEqual(howlFrameDiv(8, 2), 4.0, "8 / 2")
	assertEqual(howlFrameDiv(9.0, 2.0), 4.5, "9.0 / 2.0")
	assertEqual(howlFrameDiv(int64(9), 2.0), 4.5, "int64(9) / 2.0")
	assertPanics(func() { howlFrameDiv(1, 0) }, "1 / 0")
	assertPanics(func() { howlFrameDiv(0, 0) }, "0 / 0")

	fmt.Println("PASS")
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(mainGo), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("go", "run", path).CombinedOutput()
	if err != nil {
		t.Fatalf("go run numeric contract test failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("unexpected output: %s", out)
	}
}
