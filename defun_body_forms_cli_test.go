package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A defun body may hold several forms; the bytecode VM runs them in order.
// The JavaScript and Go backends emitted only the last form, so an early
// return was silently dropped and the targets disagreed.
func TestDefunWithSeveralBodyFormsMatchesBytecode(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	repo, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	body := `(defun ext (m)
    (type_hint m "string")
    (if (= m "image/png") (return "png"))
    (if (= m "image/jpeg") (return "jpg"))
    (return "gif"))
  (defun noisy ()
    (type_hint return "void")
    (print "one")
    (print "two"))
  (do (print (call ext "image/png")) (print (call ext "image/jpeg")) (print (call ext "x")) (call noisy))`
	want := "png\njpg\ngif\none\ntwo\n"
	dir := t.TempDir()

	cli := mustWriteFile(t, dir, "cli.howl", "(cli_app\n"+body+")\n")
	if got, err := exec.Command(bin, "run", cli).CombinedOutput(); err != nil || string(got) != want {
		t.Fatalf("bytecode: %q %v", got, err)
	}

	if _, err := exec.LookPath("node"); err == nil {
		web := mustWriteFile(t, dir, "web.howl", "(web_app\n"+body+")\n")
		out := filepath.Join(dir, "js")
		if b, err := exec.Command(bin, "build", web, "--target", "js", "-o", out).CombinedOutput(); err != nil {
			t.Fatalf("js build: %v %s", err, b)
		}
		if got, err := exec.Command("node", filepath.Join(out, "app.js")).CombinedOutput(); err != nil || string(got) != want {
			t.Fatalf("javascript printed %q (%v), want %q", got, err, want)
		}
	}

	goOut := filepath.Join(dir, "go")
	if b, err := exec.Command(bin, "build", cli, "--target", "go", "-o", goOut).CombinedOutput(); err != nil {
		t.Fatalf("go build: %v %s", err, b)
	}
	mod := "module probe\n\ngo 1.25\n\nrequire github.com/howlcipher/howlframe v0.0.0\n\nreplace github.com/howlcipher/howlframe => " + repo + "\n"
	os.WriteFile(filepath.Join(goOut, "go.mod"), []byte(mod), 0o600)
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = goOut
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	got, err := cmd.Output()
	if err != nil || string(got) != want {
		t.Fatalf("go printed %q (%v), want %q", got, err, want)
	}
}
