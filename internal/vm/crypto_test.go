package vm

import (
	"bytes"
	"strings"
	"testing"
)

// Milestone 2 token authentication needs a hash, secure randomness and a
// constant-time comparison; none of them carry authority, so no capability.
func TestCryptoBuiltinsInterpreterAndBytecode(t *testing.T) {
	const src = `(cli_app
  (print (sha256_hex "abc"))
  (print (sha256_hex ""))
  (print (secure_equal "tok" "tok") (secure_equal "tok" "tok2") (secure_equal "" "") (secure_equal "a" ""))
  (print (str_len (rand_hex 16)) (= (rand_hex 8) (rand_hex 8))))`
	want := []string{
		"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"true false true false",
		"32 false",
	}
	run := func(name string, f func() (string, int)) {
		out, code := f()
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", name, code, out)
		}
		if got := strings.Split(strings.TrimSpace(out), "\n"); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("%s: got %q want %q", name, got, want)
		}
	}
	node, prog := parseAndCompile(t, src)
	run("interpreter", func() (string, int) {
		var out, e bytes.Buffer
		c := Interpret(node, nil, nil, nil, &out, &e)
		return out.String() + e.String(), c
	})
	run("bytecode", func() (string, int) {
		var out, e bytes.Buffer
		c := RunBytecode(prog, nil, nil, nil, &out, &e)
		return out.String() + e.String(), c
	})
}

func TestRandHexBoundsAndTypes(t *testing.T) {
	for _, src := range []string{`(cli_app (print (rand_hex 0)))`, `(cli_app (print (rand_hex 65)))`} {
		_, prog := parseAndCompile(t, src)
		var out, e bytes.Buffer
		ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, nil, &out, &e, 0)
		if ev.RuntimeFailure == nil || !strings.Contains(ev.RuntimeFailure.Message, "rand_hex") {
			t.Fatalf("%s: want rand_hex failure, got %#v", src, ev.RuntimeFailure)
		}
	}
	if s, err := randHex(int64(64)); err != nil || len(s) != 128 {
		t.Fatalf("64 bytes: %q %v", s, err)
	}
}
