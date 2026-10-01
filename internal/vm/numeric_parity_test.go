package vm

import (
	"bytes"
	"strings"
	"testing"
)

// numericParityCase is one row in the cross-backend numeric semantic corpus.
// It is executed against both the direct interpreter and the production bytecode
// VM so the two canonical runtimes stay aligned.
type numericParityCase struct {
	name                   string
	source                 string
	stdin                  string
	wantOut                string
	wantErr                string // substring expected on stderr when exit != 0
	wantExit               int    // 0 for success, non-zero for expected runtime error
	interpreterUnsupported bool
}

func runNumericCase(t *testing.T, c numericParityCase) {
	t.Helper()
	node, program := parseAndCompile(t, c.source)
	runners := map[string]func(out, errOut *bytes.Buffer) int{
		"interpreter": func(out, errOut *bytes.Buffer) int {
			return Interpret(node, nil, nil, strings.NewReader(c.stdin), out, errOut)
		},
		"bytecode": func(out, errOut *bytes.Buffer) int {
			evidence := RunBytecodeWithEvidence(program, nil, DefaultExecutionPolicy(), nil, strings.NewReader(c.stdin), out, errOut, 0)
			if evidence.RuntimeFailure != nil {
				errOut.WriteString(evidence.RuntimeFailure.Code)
				if evidence.RuntimeFailure.Message != "" {
					errOut.WriteString(": " + evidence.RuntimeFailure.Message)
				}
				return 1
			}
			return evidence.ExitCode
		},
	}
	for name, run := range runners {
		if name == "interpreter" && c.interpreterUnsupported {
			continue
		}
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			exit := run(&out, &errOut)
			if c.wantExit == 0 {
				if exit != 0 {
					t.Fatalf("unexpected failure: exit=%d stdout=%q stderr=%q", exit, out.String(), errOut.String())
				}
				if out.String() != c.wantOut {
					t.Fatalf("stdout=%q, want %q; stderr=%q", out.String(), c.wantOut, errOut.String())
				}
				return
			}
			if exit == 0 {
				t.Fatalf("expected error but succeeded: stdout=%q", out.String())
			}
			if c.wantOut != "" && out.String() != c.wantOut {
				t.Fatalf("stdout=%q, want %q", out.String(), c.wantOut)
			}
			if c.wantErr != "" && !strings.Contains(errOut.String(), c.wantErr) {
				t.Fatalf("stderr=%q, want substring %q", errOut.String(), c.wantErr)
			}
		})
	}
}

func TestNumericDivisionSemantics(t *testing.T) {
	cases := []numericParityCase{
		{name: "int/int real", source: `(cli_app (print (/ 9 2)))`, stdin: "", wantOut: "4.5\n", wantErr: "", wantExit: 0},
		{name: "int/int exact", source: `(cli_app (print (/ 8 2)))`, stdin: "", wantOut: "4\n", wantErr: "", wantExit: 0},
		{name: "float/float", source: `(cli_app (print (/ 9.0 2.0)))`, stdin: "", wantOut: "4.5\n", wantErr: "", wantExit: 0},
		{name: "mixed operands", source: `(cli_app (print (/ 9 2.0)))`, stdin: "", wantOut: "4.5\n", wantErr: "", wantExit: 0},
		{name: "zero numerator", source: `(cli_app (print (/ 0 5)))`, stdin: "", wantOut: "0\n", wantErr: "", wantExit: 0},
		{name: "division by zero", source: `(cli_app (print (/ 1 0)))`, stdin: "", wantOut: "", wantErr: "division by zero", wantExit: 1},
		{name: "negative division", source: `(cli_app (print (/ (- 0 9) 2)))`, stdin: "", wantOut: "-4.5\n", wantErr: "", wantExit: 0},
		{name: "division decision threshold", source: `(cli_app (if (> (/ 9 2) 4) (print "ALLOW") (print "DENY")))`, stdin: "", wantOut: "ALLOW\n", wantErr: "", wantExit: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runNumericCase(t, c) })
	}
}

func TestNumericMixedOrdering(t *testing.T) {
	cases := []numericParityCase{
		{name: "2^53+1 > 2^53.0", source: `(cli_app (print (> 9007199254740993 9007199254740992.0)))`, stdin: "", wantOut: "true\n", wantErr: "", wantExit: 0},
		{name: "2^53+1 not eq 2^53.0", source: `(cli_app (print (== 9007199254740993 9007199254740992.0)))`, stdin: "", wantOut: "false\n", wantErr: "", wantExit: 0},
		{name: "2^53+1 != 2^53.0", source: `(cli_app (print (!= 9007199254740993 9007199254740992.0)))`, stdin: "", wantOut: "true\n", wantErr: "", wantExit: 0},
		{name: "list_len vs float", source: `(cli_app (let (xs (list 1 2)) (print (< (list_len xs) 2.5))))`, stdin: "", wantOut: "true\n", wantErr: "", wantExit: 0},
		{name: "boundary equality", source: `(cli_app (print (== 9007199254740992 9007199254740992.0)))`, stdin: "", wantOut: "true\n", wantErr: "", wantExit: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runNumericCase(t, c) })
	}
}

func TestNumericArithmeticOverflow(t *testing.T) {
	cases := []numericParityCase{
		{name: "add overflow", source: `(cli_app (print (+ 9223372036854775807 1)))`, stdin: "", wantOut: "", wantErr: "integer overflow", wantExit: 1},
		{name: "sub to min", source: `(cli_app (let (n (- 0 9223372036854775807)) (print (- n 2))))`, stdin: "", wantOut: "", wantErr: "integer overflow", wantExit: 1},
		{name: "mul overflow", source: `(cli_app (print (* 3037000500 3037000500)))`, stdin: "", wantOut: "", wantErr: "integer overflow", wantExit: 1},
		{name: "large safe add", source: `(cli_app (print (+ 9007199254740992 1)))`, stdin: "", wantOut: "9007199254740993\n", wantErr: "", wantExit: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runNumericCase(t, c) })
	}
}

func TestNumericConversionsFailClosedParity(t *testing.T) {
	cases := []numericParityCase{
		{name: "to_int invalid string", source: `(cli_app (print (to_int "abc")))`, stdin: "", wantOut: "", wantErr: "CONVERSION_ERROR", wantExit: 1},
		{name: "to_int empty", source: `(cli_app (print (to_int "")))`, stdin: "", wantOut: "", wantErr: "CONVERSION_ERROR", wantExit: 1},
		{name: "to_int overflow", source: `(cli_app (print (to_int "9223372036854775808")))`, stdin: "", wantOut: "", wantErr: "CONVERSION_ERROR", wantExit: 1},
		{name: "to_float invalid", source: `(cli_app (print (to_float "abc")))`, stdin: "", wantOut: "", wantErr: "CONVERSION_ERROR", wantExit: 1},
		{name: "to_float overflow", source: `(cli_app (print (to_float "1e9999")))`, stdin: "", wantOut: "", wantErr: "CONVERSION_ERROR", wantExit: 1},
		{name: "to_int truncates", source: `(cli_app (print (to_int 3.9)))`, stdin: "", wantOut: "3\n", wantErr: "", wantExit: 0},
		{name: "to_float from int", source: `(cli_app (print (to_float 7)))`, stdin: "", wantOut: "7\n", wantErr: "", wantExit: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runNumericCase(t, c) })
	}
}

func TestJSONNumericSemantics(t *testing.T) {
	cases := []numericParityCase{
		{
			name:                   "json large integers stay exact",
			source:                 `(cli_app (let (line (read_line)) (let (ctx (parse_json Context line)) (do (print (= (map_get ctx "a") 9007199254740993)) (print (= (map_get ctx "a") 9007199254740992))))))))`,
			stdin:                  `{"a":9007199254740993}` + "\n",
			wantOut:                "true\nfalse\n",
			wantErr:                "",
			wantExit:               0,
			interpreterUnsupported: true,
		},
		{
			name:                   "json 2^53+1 distinct from 2^53",
			source:                 `(cli_app (let (line (read_line)) (let (ctx (parse_json Context line)) (print (!= (map_get ctx "x") (map_get ctx "y"))))))`,
			stdin:                  `{"x":9007199254740993,"y":9007199254740992}` + "\n",
			wantOut:                "true\n",
			wantErr:                "",
			wantExit:               0,
			interpreterUnsupported: true,
		},
		{
			name:                   "json count equals list_len",
			source:                 `(cli_app (let (line (read_line)) (let (ctx (parse_json Context line)) (print (= (map_get ctx "count") (list_len (map_get ctx "items"))))))))`,
			stdin:                  `{"items":[1,2,3],"count":3}` + "\n",
			wantOut:                "true\n",
			wantErr:                "",
			wantExit:               0,
			interpreterUnsupported: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runNumericCase(t, c) })
	}
}

func TestNumericBuiltinParity(t *testing.T) {
	cases := []numericParityCase{
		{name: "list_len equality", source: `(cli_app (print (= (list_len (list 1 2 3)) 3)))`, stdin: "", wantOut: "true\n", wantErr: "", wantExit: 0},
		{name: "list_len ordering", source: `(cli_app (print (< (list_len (list)) 1)))`, stdin: "", wantOut: "true\n", wantErr: "", wantExit: 0},
		{name: "list_len arithmetic", source: `(cli_app (print (+ (list_len (list 1 2)) 1)))`, stdin: "", wantOut: "3\n", wantErr: "", wantExit: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runNumericCase(t, c) })
	}
}

func TestFalseAllowNumericRegressions(t *testing.T) {
	cases := []numericParityCase{
		{
			name:    "threshold allow",
			source:  `(cli_app (let (usage 7) (let (quota 10) (if (> (/ usage quota) 0.75) (print "DENY") (print "ALLOW")))))`,
			wantOut: "ALLOW\n",
		},
		{
			name:    "threshold deny",
			source:  `(cli_app (let (usage 8) (let (quota 10) (if (> (/ usage quota) 0.75) (print "DENY") (print "ALLOW")))))`,
			wantOut: "DENY\n",
		},
		{
			name:    "minimum approval count",
			source:  `(cli_app (let (approvers (list "a" "b" "c")) (let (required 3) (if (< (list_len approvers) required) (print "DENY") (print "ALLOW")))))`,
			wantOut: "ALLOW\n",
		},
		{
			name:    "one short approval count",
			source:  `(cli_app (let (approvers (list "a" "b")) (let (required 3) (if (< (list_len approvers) required) (print "DENY") (print "ALLOW")))))`,
			wantOut: "DENY\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runNumericCase(t, c) })
	}
}
