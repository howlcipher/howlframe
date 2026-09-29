package vm

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/htmlescape"
)

const htmlEscapeWant = "&amp;\n" +
	"&lt;\n" +
	"&gt;\n" +
	"&#34;\n" +
	"&#39;\n" +
	"&amp;&lt;&gt;&#34;&#39;\n" +
	"&amp;&lt;&gt;&#34;&#39;\n" +
	"[]\n" +
	"<button data-id=\"a&lt;b&amp;c&#34;d&#39;e\" onclick=\"onTask\">x&lt;y&amp;z</button>\n"

func TestHTMLEscapeMatchesOnInterpreterAndBytecode(t *testing.T) {
	source := string(mustRead(t, "../../tests/parity/13_html_escape.howl"))

	t.Run("interpreter", func(t *testing.T) {
		node, _ := parseAndCompile(t, source)
		var out, errOut bytes.Buffer
		if exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != htmlEscapeWant {
			t.Fatalf("stdout = %q, want %q", out.String(), htmlEscapeWant)
		}
	})

	t.Run("bytecode", func(t *testing.T) {
		_, prog := parseAndCompile(t, source)
		var out, errOut bytes.Buffer
		if exitCode := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != htmlEscapeWant {
			t.Fatalf("stdout = %q, want %q", out.String(), htmlEscapeWant)
		}
		if strings.Contains(out.String(), "a<b") || strings.Contains(out.String(), "<script") {
			t.Fatalf("raw markup survived encoding:\n%s", out.String())
		}
	})
}

func TestHTMLEscapeRejectsNonString(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{name: "html_escape list", source: `(cli_app (print (html_escape (list "a"))))`, want: "html_escape expected string"},
		{name: "attr_escape number", source: `(cli_app (print (attr_escape 1)))`, want: "attr_escape expected string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("interpreter", func(t *testing.T) {
				node, _ := parseAndCompile(t, tc.source)
				var out, errOut bytes.Buffer
				exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut)
				if exitCode == 0 || !strings.Contains(errOut.String(), "TYPE_ERROR") || !strings.Contains(errOut.String(), tc.want) {
					t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, out.String(), errOut.String())
				}
			})
			t.Run("bytecode", func(t *testing.T) {
				_, prog := parseAndCompile(t, tc.source)
				var out, errOut bytes.Buffer
				evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &errOut, 0)
				if evidence.RuntimeFailure == nil || evidence.RuntimeFailure.Code != "TYPE_ERROR" {
					t.Fatalf("failure = %#v, want TYPE_ERROR", evidence.RuntimeFailure)
				}
				if !strings.Contains(evidence.RuntimeFailure.Message, tc.want) {
					t.Fatalf("message = %q", evidence.RuntimeFailure.Message)
				}
			})
		})
	}
}

func TestHTMLEscapeHasNoCapability(t *testing.T) {
	for _, op := range []bytecode.Opcode{bytecode.OpHTMLEscape, bytecode.OpAttrEscape} {
		spec := bytecode.Registry[op]
		if spec.Capability != capability.None {
			t.Fatalf("%s capability = %q, want none", spec.Name, spec.Capability)
		}
		if spec.Capability == capability.Network || spec.Capability == capability.Database {
			t.Fatalf("%s granted %q", spec.Name, spec.Capability)
		}
	}
	if capability.ForConstruct("html_escape") != capability.None || capability.ForConstruct("attr_escape") != capability.None {
		t.Fatal("escape constructs must not declare a capability")
	}
	source := string(mustRead(t, "../../tests/parity/13_html_escape.howl"))
	_, prog := parseAndCompile(t, source)
	var out, errOut bytes.Buffer
	if exitCode := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
		t.Fatalf("empty grant rejected a pure escape: exit = %d, stderr = %s", exitCode, errOut.String())
	}
}

func TestHandlerEncodeContractOnMarkupProgram(t *testing.T) {
	source := string(mustRead(t, "../../tests/parity/13_html_escape.howl"))
	node, _ := parseAndCompile(t, source)
	pieces, ok := htmlescape.ProgramMarkupPieces(node)
	if !ok {
		t.Fatal("parity fixture has no markup join")
	}
	names, violation := htmlescape.ScanHandlers(pieces)
	if violation != "" {
		t.Fatalf("handler contract: %s", violation)
	}
	if len(names) != 1 || names[0] != "onTask" {
		t.Fatalf("handler names = %#v, want constant onTask", names)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
