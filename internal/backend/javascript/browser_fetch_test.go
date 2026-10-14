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
		{"coarse network (#165)", `"network"`, false},
		{"other host", `"network:fetch=example.com"`, false},
		{"suffix host", `"network:fetch=invalid"`, false},
		{"other port", `"network:fetch=example.invalid:8443"`, false},
		{"host grant", `"network:fetch=example.invalid"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// vm supplies browser globals without Node's process or require.
			wrapper := `const vm = require("node:vm");
let calls = 0;
const logs = [];
const browser = {
  URL,
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
	wrapper := `globalThis.HOWLFRAME_ALLOW_CAPS = "network:fetch=example.invalid";
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
  URL,
  HOWLFRAME_ALLOW_CAPS: "network:fetch=example.invalid",
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

func TestBrowserRelativeFetchChecksResolvedHost(t *testing.T) {
	for _, tc := range []struct {
		name, url, grant, base string
		allowed                bool
		wantURL                string
	}{
		{"same origin", "missions.json", "network:fetch=example.invalid", "https://example.invalid/docs/index.html", true, "https://example.invalid/docs/missions.json"},
		{"relative path", "../missions.json", "network:fetch=example.invalid", "https://example.invalid/docs/index.html", true, "https://example.invalid/missions.json"},
		{"other host", "//evil.invalid/missions.json", "network:fetch=example.invalid", "https://example.invalid/docs/index.html", false, ""},
		{"coarse grant", "missions.json", "network", "https://example.invalid/docs/index.html", false, ""},
		{"wrong port", "missions.json", "network:fetch=example.invalid:8443", "https://example.invalid/docs/index.html", false, ""},
		{"no browser base", "missions.json", "network:fetch=example.invalid", "", false, ""},
		{"credential base", "missions.json", "network:fetch=example.invalid", "https://user:pw@example.invalid/docs/", false, ""},
		{"file base", "missions.json", "network:fetch=example.invalid", "file:///tmp/index.html", false, ""},
		{"explicit port", "missions.json", "network:fetch=example.invalid:8443", "https://example.invalid:8443/docs/", true, "https://example.invalid:8443/docs/missions.json"},
		{"ungranted port", "missions.json", "network:fetch=example.invalid", "https://example.invalid:8443/docs/", false, ""},
		{"absolute otherhost", "https://evil.invalid/x", "network:fetch=example.invalid", "https://example.invalid/docs/", false, ""},
		{"three slash host", "///evil.invalid/x", "network:fetch=example.invalid", "https://example.invalid/docs/", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `(web_app (print (bytes_to_string (fetch "` + tc.url + `" "GET"))))`
			root := parser.NewParser(lexer.NewLexer(src), "relative.howl").ParseExpression()
			checker.Check(root)
			code, _ := GenerateJSCode(root)
			encoded, _ := json.Marshal(code)
			grant, _ := json.Marshal(tc.grant)
			base, _ := json.Marshal(tc.base)
			wrapper := `const vm=require("node:vm"); let calls=0; let seen=""; const browser={URL,HOWLFRAME_ALLOW_CAPS:` + string(grant) + `,console:{log:()=>{},error:()=>{}},fetch:async u=>{calls++;seen=String(u);return{text:async()=>"ok"}}};
const base=` + string(base) + `;if(base)browser.location={href:base};
Promise.resolve(vm.runInNewContext(` + string(encoded) + `,browser)).then(()=>console.log(JSON.stringify({calls,seen,error:""})),e=>console.log(JSON.stringify({calls,seen,error:String(e.message)})));`
			stdout, stderr, exit := runJS(t, wrapper)
			if exit != 0 {
				t.Fatalf("harness: %s %s", stdout, stderr)
			}
			var result struct {
				Calls int
				Seen  string
				Error string
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if tc.allowed {
				if result.Calls != 1 || result.Error != "" || result.Seen != tc.wantURL {
					t.Fatalf("relative browser fetch failed: %+v", result)
				}
			} else if result.Calls != 0 || !strings.Contains(result.Error, "CAPABILITY_DENIED") {
				t.Fatalf("must deny before request: %+v", result)
			}
		})
	}
}

func TestNodeRelativeFetchHasNoBrowserBase(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(web_app (print (bytes_to_string (fetch "missions.json" "GET"))))`), "node-relative.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateJSCode(root)
	wrapper := `process.env.HOWLFRAME_ALLOW_CAPS="network:fetch=example.invalid";
 globalThis.location={href:"https://example.invalid/docs/index.html"};
 globalThis.fetch=()=>{throw new Error("UNEXPECTED_REQUEST")};
` + code
	stdout, stderr, exit := runJS(t, wrapper)
	if exit == 0 || stdout != "" || !strings.Contains(stderr, "CAPABILITY_DENIED") || strings.Contains(stderr, "UNEXPECTED_REQUEST") {
		t.Fatalf("Node gained implicit browser base: exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}
