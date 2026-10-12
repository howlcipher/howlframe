package javascript

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestBrowserFetchRequiresExplicitHostGrant(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(web_app (print (bytes_to_string (fetch "https://example.invalid" "GET"))))`), "browser.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	encoded, err := json.Marshal(code)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, grant string
		allowed     bool
	}{
		{"unset", "undefined", false},
		{"empty", `""`, false},
		{"wrong", `"filesystem"`, false},
		{"invalid", "42", false},
		{"network", `"network"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// vm supplies browser globals without Node's process or require.
			wrapper := `const vm = require("node:vm");
let calls = 0;
const logs = [];
const browser = {
  console: {log: value => logs.push(String(value)), error: () => {}},
  fetch: async () => { calls++; return {text: async () => "browser response"}; }
};
const grant = ` + tc.grant + `;
if (grant !== undefined) browser.HOWLFRAME_ALLOW_CAPS = grant;
Promise.resolve(vm.runInNewContext(` + string(encoded) + `, browser)).then(
  () => console.log(JSON.stringify({calls, logs, error: ""})),
  err => console.log(JSON.stringify({calls, logs, error: String(err.message)}))
);`
			stdout, stderr, exit := runJS(t, wrapper)
			if exit != 0 {
				t.Fatalf("browser harness failed: %s %s", stdout, stderr)
			}
			var result struct {
				Calls int      `json:"calls"`
				Logs  []string `json:"logs"`
				Error string   `json:"error"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("invalid browser result %q: %v", stdout, err)
			}
			if tc.allowed {
				if result.Calls != 1 || result.Error != "" || len(result.Logs) != 1 || result.Logs[0] != "browser response" {
					t.Fatalf("granted fetch failed: %+v", result)
				}
			} else if result.Calls != 0 || len(result.Logs) != 0 || !strings.Contains(result.Error, "CAPABILITY_DENIED") {
				t.Fatalf("fetch must fail closed before dialing: %+v", result)
			}
		})
	}
}

func TestNodeEnvironmentCannotBeWidenedByBrowserGrant(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(web_app (print (bytes_to_string (fetch "https://example.invalid" "GET"))))`), "node.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	wrapper := `globalThis.HOWLFRAME_ALLOW_CAPS = "network";
globalThis.fetch = () => { throw new Error("UNEXPECTED_REQUEST"); };
` + code
	stdout, stderr, exit := runJS(t, wrapper, "HOWLFRAME_ALLOW_CAPS=")
	if exit == 0 || !strings.Contains(stderr, "CAPABILITY_DENIED") || strings.Contains(stdout+stderr, "UNEXPECTED_REQUEST") {
		t.Fatalf("browser global widened Node authority: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}

func TestBrowserBootstrapPreservesOriginalError(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(web_app (print (bytes_to_string (fetch "https://example.invalid" "GET"))))`), "browser-error.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	encoded, err := json.Marshal(code)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := `const vm = require("node:vm");
const original = new Error("NETWORK_FAILED");
const browser = {
  HOWLFRAME_ALLOW_CAPS: "network",
  console: {log: () => {}, error: () => {}},
  fetch: async () => { throw original; }
};
Promise.resolve(vm.runInNewContext(` + string(encoded) + `, browser)).then(
  () => { throw new Error("UNEXPECTED_SUCCESS"); },
  err => { if (err !== original) throw new Error("ERROR_REPLACED"); console.log("original error preserved"); }
);`
	stdout, stderr, exit := runJS(t, wrapper)
	if exit != 0 || strings.TrimSpace(stdout) != "original error preserved" {
		t.Fatalf("bootstrap replaced the browser error: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}
