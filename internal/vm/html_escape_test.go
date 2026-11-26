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

	t.Run("interpreter", func(t *testing.T) { assertInterpretStdout(t, source, htmlEscapeWant) })

	t.Run("bytecode", func(t *testing.T) {
		got := assertBytecodeStdout(t, source, htmlEscapeWant)
		if strings.Contains(got, "a<b") || strings.Contains(got, "<script") {
			t.Fatalf("raw markup survived encoding:\n%s", got)
		}
	})
}

func TestHTMLEscapeRejectsNonString(t *testing.T) {
	runVMTypeErrorCases(t, []vmTypeErrCase{
		{"html_escape list", `(cli_app (print (html_escape (list "a"))))`, "html_escape expected string"},
		{"attr_escape number", `(cli_app (print (attr_escape 1)))`, "attr_escape expected string"},
	})
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
