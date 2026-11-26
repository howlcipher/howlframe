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

func numOK(name, source, wantOut string) numericParityCase {
	return numericParityCase{name: name, source: source, wantOut: wantOut}
}

func numFail(name, source, wantErr string) numericParityCase {
	return numericParityCase{name: name, source: source, wantErr: wantErr, wantExit: 1}
}

func runNumericCases(t *testing.T, cases []numericParityCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runNumericCase(t, c) })
	}
}

func TestNumericDivisionSemantics(t *testing.T) {
	runNumericCases(t, []numericParityCase{
		numOK("int/int real", `(cli_app (print (/ 9 2)))`, "4.5\n"),
		numOK("int/int exact", `(cli_app (print (/ 8 2)))`, "4\n"),
		numOK("float/float", `(cli_app (print (/ 9.0 2.0)))`, "4.5\n"),
		numOK("mixed operands", `(cli_app (print (/ 9 2.0)))`, "4.5\n"),
		numOK("zero numerator", `(cli_app (print (/ 0 5)))`, "0\n"),
		numFail("division by zero", `(cli_app (print (/ 1 0)))`, "division by zero"),
		numOK("negative division", `(cli_app (print (/ (- 0 9) 2)))`, "-4.5\n"),
		numOK("division decision threshold", `(cli_app (if (> (/ 9 2) 4) (print "ALLOW") (print "DENY")))`, "ALLOW\n"),
	})
}

func TestNumericMixedOrdering(t *testing.T) {
	runNumericCases(t, []numericParityCase{
		numOK("2^53+1 > 2^53.0", `(cli_app (print (> 9007199254740993 9007199254740992.0)))`, "true\n"),
		numOK("2^53+1 not eq 2^53.0", `(cli_app (print (== 9007199254740993 9007199254740992.0)))`, "false\n"),
		numOK("2^53+1 != 2^53.0", `(cli_app (print (!= 9007199254740993 9007199254740992.0)))`, "true\n"),
		numOK("list_len vs float", `(cli_app (let (xs (list 1 2)) (print (< (list_len xs) 2.5))))`, "true\n"),
		numOK("boundary equality", `(cli_app (print (== 9007199254740992 9007199254740992.0)))`, "true\n"),
	})
}

func TestNumericArithmeticOverflow(t *testing.T) {
	runNumericCases(t, []numericParityCase{
		numFail("add overflow", `(cli_app (print (+ 9223372036854775807 1)))`, "integer overflow"),
		numFail("sub to min", `(cli_app (let (n (- 0 9223372036854775807)) (print (- n 2))))`, "integer overflow"),
		numFail("mul overflow", `(cli_app (print (* 3037000500 3037000500)))`, "integer overflow"),
		numOK("large safe add", `(cli_app (print (+ 9007199254740992 1)))`, "9007199254740993\n"),
	})
}

func TestNumericConversionsFailClosedParity(t *testing.T) {
	runNumericCases(t, []numericParityCase{
		numFail("to_int invalid string", `(cli_app (print (to_int "abc")))`, "CONVERSION_ERROR"),
		numFail("to_int empty", `(cli_app (print (to_int "")))`, "CONVERSION_ERROR"),
		numFail("to_int overflow", `(cli_app (print (to_int "9223372036854775808")))`, "CONVERSION_ERROR"),
		numFail("to_float invalid", `(cli_app (print (to_float "abc")))`, "CONVERSION_ERROR"),
		numFail("to_float overflow", `(cli_app (print (to_float "1e9999")))`, "CONVERSION_ERROR"),
		numOK("to_int truncates", `(cli_app (print (to_int 3.9)))`, "3\n"),
		numOK("to_float from int", `(cli_app (print (to_float 7)))`, "7\n"),
	})
}

func TestJSONNumericSemantics(t *testing.T) {
	runNumericCases(t, []numericParityCase{
		{
			name:                   "json large integers stay exact",
			source:                 `(cli_app (let (line (read_line)) (let (ctx (parse_json Context line)) (do (print (= (map_get ctx "a") 9007199254740993)) (print (= (map_get ctx "a") 9007199254740992))))))))`,
			stdin:                  `{"a":9007199254740993}` + "\n",
			wantOut:                "true\nfalse\n",
			interpreterUnsupported: true,
		},
		{
			name:                   "json 2^53+1 distinct from 2^53",
			source:                 `(cli_app (let (line (read_line)) (let (ctx (parse_json Context line)) (print (!= (map_get ctx "x") (map_get ctx "y"))))))`,
			stdin:                  `{"x":9007199254740993,"y":9007199254740992}` + "\n",
			wantOut:                "true\n",
			interpreterUnsupported: true,
		},
		{
			name:                   "json count equals list_len",
			source:                 `(cli_app (let (line (read_line)) (let (ctx (parse_json Context line)) (print (= (map_get ctx "count") (list_len (map_get ctx "items"))))))))`,
			stdin:                  `{"items":[1,2,3],"count":3}` + "\n",
			wantOut:                "true\n",
			interpreterUnsupported: true,
		},
	})
}

func TestNumericBuiltinParity(t *testing.T) {
	runNumericCases(t, []numericParityCase{
		numOK("list_len equality", `(cli_app (print (= (list_len (list 1 2 3)) 3)))`, "true\n"),
		numOK("list_len ordering", `(cli_app (print (< (list_len (list)) 1)))`, "true\n"),
		numOK("list_len arithmetic", `(cli_app (print (+ (list_len (list 1 2)) 1)))`, "3\n"),
	})
}

func TestFalseAllowNumericRegressions(t *testing.T) {
	runNumericCases(t, []numericParityCase{
		numOK("threshold allow", `(cli_app (let (usage 7) (let (quota 10) (if (> (/ usage quota) 0.75) (print "DENY") (print "ALLOW")))))`, "ALLOW\n"),
		numOK("threshold deny", `(cli_app (let (usage 8) (let (quota 10) (if (> (/ usage quota) 0.75) (print "DENY") (print "ALLOW")))))`, "DENY\n"),
		numOK("minimum approval count", `(cli_app (let (approvers (list "a" "b" "c")) (let (required 3) (if (< (list_len approvers) required) (print "DENY") (print "ALLOW")))))`, "ALLOW\n"),
		numOK("one short approval count", `(cli_app (let (approvers (list "a" "b")) (let (required 3) (if (< (list_len approvers) required) (print "DENY") (print "ALLOW")))))`, "DENY\n"),
	})
}
