package vm

import (
	"bytes"
	"strings"
	"testing"
)

// unicode17Cased is every code point Unicode 17.0.0 (Go 1.27) gives a case
// mapping that Unicode 15.0.0 (Go 1.25 and 1.26) does not: 110 in all,
// none of them mapped in Unicode 15.
func unicode17Cased() string {
	var b strings.Builder
	for _, r := range [][2]rune{
		{0x019B, 0x019B}, {0x0264, 0x0264}, {0x1C89, 0x1C8A}, {0xA7CB, 0xA7CF}, {0xA7D2, 0xA7D5},
		{0xA7DA, 0xA7DC}, {0x10D50, 0x10D65}, {0x10D70, 0x10D85}, {0x16EA0, 0x16EB8}, {0x16EBB, 0x16ED3},
	} {
		for c := r[0]; c <= r[1]; c++ {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// to_lower and to_upper map case by the VM's own Unicode 15.0.0 table, so a
// VM built with Go 1.27 prints what a release built with Go 1.25 prints
// (bugs.md #69). Go 1.27's strings.ToUpper maps U+0264 to U+A7CB.
func TestCaseMappingDoesNotDependOnTheToolchain(t *testing.T) {
	text := unicode17Cased()
	if n := len([]rune(text)); n != 110 {
		t.Fatalf("%d code points, want 110", n)
	}
	source := `(cli_app (let (s "` + text + `") (do (print (to_lower s)) (print (to_upper s)) (print (to_upper "Straße ǅ")))))`
	want := text + "\n" + text + "\nSTRAßE Ǆ\n"
	node, prog := parseAndCompile(t, source)
	for _, mode := range []struct {
		name string
		run  func(out, errOut *bytes.Buffer) int
	}{
		{"interpreter", func(out, errOut *bytes.Buffer) int {
			return Interpret(node, nil, nil, strings.NewReader(""), out, errOut)
		}},
		{"bytecode", func(out, errOut *bytes.Buffer) int {
			return RunBytecode(prog, nil, nil, strings.NewReader(""), out, errOut)
		}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if exit := mode.run(&out, &errOut); exit != 0 {
				t.Fatalf("exit = %d, stderr = %s", exit, errOut.String())
			}
			if out.String() != want {
				t.Fatalf("stdout = %+q, want %+q", out.String(), want)
			}
		})
	}
}
