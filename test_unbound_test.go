package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestUnboundVariablesChecker(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string // substring in error output expected; empty string means we expect success
	}{
		{"normal expression", `(cli_app (print unbound_var))`, "undefined reference"},
		{"let body", `(cli_app (let (x 1) (print unbound_var)))`, "undefined reference"},
		{"function", `(defun test () (print unbound_var))`, "undefined reference"},
		{"loop", `(cli_app (while true (print unbound_var)))`, "undefined reference"},
		{"conditional", `(cli_app (if true (print unbound_var)))`, "undefined reference"},
		{"try_let catch", `(cli_app (try_let (x 1) (catch err (print unbound_var)) (print x)))`, "undefined reference"},
		// Legitimately bound dynamic values should not error:
		{"try_let bound", `(cli_app (try_let (x (read_file "foo")) (catch err (print err)) (print x)))`, ""},
		{"parse_json", `(cli_app (let (x (parse_json "{}")) (print x)))`, ""},
		{"map get", `(cli_app (let (x (dict)) (let (y (map_get x "key")) (print y))))`, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.WriteFile("test_unbound.howl", []byte(tc.src), 0644)
			cmd := exec.Command("go", "run", "howlframe.go", "-validate", "test_unbound.howl")
			out, err := cmd.CombinedOutput()

			if tc.want == "" {
				if err != nil {
					t.Fatalf("expected success, got error: %s", out)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error, got nil. output: %s", out)
				}
				if !strings.Contains(string(out), tc.want) {
					t.Fatalf("expected error containing %q, got: %s", tc.want, out)
				}
			}
		})
	}
}
