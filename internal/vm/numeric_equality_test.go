package vm

import (
	"bytes"
	"strings"
	"testing"
)

// list_len and time_now produce int64 while literals, arithmetic and JSON
// produce float64 on the VM. Equality must compare them by value, or a guard
// such as "deny when the approver list is empty" silently never fires.
const numericEqualitySource = `(cli_app
  (let (approvers (list))
    (do
      (print "empty-eq-0" (= (list_len approvers) 0))
      (print "two-eq-2" (= (list_len (list 1 2)) 2))
      (print "two-ne-2" (!= (list_len (list 1 2)) 2))
      (print "two-eq-3" (= (list_len (list 1 2)) 3))
      (print "to-int-len" (= (to_int (list_len (list 1))) 1))
      (if (= (list_len approvers) 0) (print "decision DENY") (print "decision ALLOW")))))`

const numericEqualityWant = "empty-eq-0 true\n" +
	"two-eq-2 true\n" +
	"two-ne-2 false\n" +
	"two-eq-3 false\n" +
	"to-int-len true\n" +
	"decision DENY\n"

func TestNumericEqualityAcrossRepresentations(t *testing.T) {
	t.Run("interpreter", func(t *testing.T) {
		node, _ := parseAndCompile(t, numericEqualitySource)
		var out, errOut bytes.Buffer
		if exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != numericEqualityWant {
			t.Fatalf("stdout = %q, want %q", out.String(), numericEqualityWant)
		}
	})

	t.Run("bytecode", func(t *testing.T) {
		_, prog := parseAndCompile(t, numericEqualitySource)
		var out, errOut bytes.Buffer
		if exitCode := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != numericEqualityWant {
			t.Fatalf("stdout = %q, want %q", out.String(), numericEqualityWant)
		}
	})
}

func TestJSONNumberEqualsListLen(t *testing.T) {
	const src = `(cli_app
  (let (line (read_line))
    (let (ctx (parse_json Ctx line))
      (let (items (map_get ctx "items"))
        (print "count-eq-len" (= (map_get ctx "count") (list_len items)))))))`
	_, prog := parseAndCompile(t, src)
	var out, errOut bytes.Buffer
	if exitCode := RunBytecode(prog, nil, nil, strings.NewReader(`{"items":[1,2],"count":2}`+"\n"), &out, &errOut); exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
	}
	if out.String() != "count-eq-len true\n" {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestJSONIntegersRemainExactAcrossFloatBoundary(t *testing.T) {
	const source = `(cli_app
  (let (line (read_line))
    (let (ctx (parse_json Context line))
      (do
        (print (= (map_get ctx "below") 9007199254740991))
        (print (= (map_get ctx "boundary") (map_get ctx "above")))
        (print (!= (map_get ctx "boundary") (map_get ctx "above")))))))`
	_, program := parseAndCompile(t, source)
	var out, errOut bytes.Buffer
	input := `{"below":9007199254740991,"boundary":9007199254740992,"above":9007199254740993}` + "\n"
	if exit := RunBytecode(program, nil, nil, strings.NewReader(input), &out, &errOut); exit != 0 {
		t.Fatalf("exit = %d, stderr = %s", exit, errOut.String())
	}
	if want := "true\nfalse\ntrue\n"; out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

func TestNumericEqualIsExact(t *testing.T) {
	cases := []struct {
		a, b any
		want bool
	}{
		{int64(0), float64(0), true},
		{float64(2), int64(2), true},
		{int64(2), float64(2.5), false},
		{int64(9007199254740993), float64(9007199254740992), false},
		{int64(3), int64(3), true},
		{float64(1.5), float64(1.5), true},
	}
	for _, c := range cases {
		if got := BcValuesEqual(c.a, c.b); got != c.want {
			t.Errorf("BcValuesEqual(%T %v, %T %v) = %v, want %v", c.a, c.a, c.b, c.b, got, c.want)
		}
		if got := ValuesEqual(c.a, c.b); got != c.want {
			t.Errorf("ValuesEqual(%T %v, %T %v) = %v, want %v", c.a, c.a, c.b, c.b, got, c.want)
		}
	}
	if BcValuesEqual("1", float64(1)) {
		t.Error("a string must never equal a number")
	}
}
