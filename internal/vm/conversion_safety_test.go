package vm

import (
	"bytes"
	"strings"
	"testing"
)

func TestNumericConversionsAgreeOnValidInputs(t *testing.T) {
	const source = `(cli_app
  (print (to_int "42") (to_int " -7 ") (to_int 3.9))
  (print (to_float "2.5") (to_float (list_len (list 1 2)))))`
	const want = "42 -7 3\n2.5 2\n"
	node, program := parseAndCompile(t, source)
	for name, run := range map[string]func(*bytes.Buffer, *bytes.Buffer) int{
		"interpreter": func(out, errOut *bytes.Buffer) int {
			return Interpret(node, nil, nil, strings.NewReader(""), out, errOut)
		},
		"bytecode": func(out, errOut *bytes.Buffer) int {
			return RunBytecode(program, nil, nil, strings.NewReader(""), out, errOut)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if exit := run(&out, &errOut); exit != 0 || out.String() != want {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", exit, out.String(), errOut.String())
			}
		})
	}
}

func TestNumericConversionsFailClosed(t *testing.T) {
	cases := []string{
		`(to_int "abc")`,
		`(to_int "")`,
		`(to_int "9223372036854775808")`,
		`(to_float "abc")`,
		`(to_float "1e9999")`,
	}
	for _, expression := range cases {
		t.Run(expression, func(t *testing.T) {
			node, program := parseAndCompile(t, `(cli_app (print `+expression+`))`)
			for name, run := range map[string]func(*bytes.Buffer) int{
				"interpreter": func(errOut *bytes.Buffer) int {
					return Interpret(node, nil, nil, strings.NewReader(""), &bytes.Buffer{}, errOut)
				},
				"bytecode": func(errOut *bytes.Buffer) int {
					evidence := RunBytecodeWithEvidence(program, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &bytes.Buffer{}, errOut, 0)
					if evidence.RuntimeFailure != nil {
						errOut.WriteString(evidence.RuntimeFailure.Code)
						return 1
					}
					return evidence.ExitCode
				},
			} {
				t.Run(name, func(t *testing.T) {
					var errOut bytes.Buffer
					if exit := run(&errOut); exit == 0 || !strings.Contains(errOut.String(), "CONVERSION_ERROR") {
						t.Fatalf("exit = %d, stderr = %q", exit, errOut.String())
					}
				})
			}
		})
	}
}
