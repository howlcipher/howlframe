package bytecode

import (
	"encoding/json"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"reflect"
	"testing"
)

func TestRequiredCapabilities(t *testing.T) {
	tests := []struct {
		name string
		p    *BCProgram
		want []string
	}{
		{"empty", &BCProgram{}, []string{}},
		{"memory", &BCProgram{Main: []BCInstruction{{Op: OpStoreOpen, StringOperand2: "memory://x"}}}, []string{"database"}},
		{"file", &BCProgram{Main: []BCInstruction{{Op: OpStoreOpen, StringOperand2: "file://x"}}}, []string{"database", "filesystem"}},
		{"unknown URI", &BCProgram{Main: []BCInstruction{{Op: OpStoreOpen}}}, []string{"database", "filesystem"}},
		{"lazy", &BCProgram{Main: []BCInstruction{{Op: OpCall, StringOperand: "f"}}, Functions: map[string]*BCFunction{"f": {LazySynthesize: true}}}, []string{"database", "environment", "filesystem", "network", "process"}},
		{"unknown call", &BCProgram{Main: []BCInstruction{{Op: OpCall}}}, []string{"database", "environment", "filesystem", "network", "process"}},
		{"ordinary call", &BCProgram{Main: []BCInstruction{{Op: OpCall, StringOperand: "f"}}, Functions: map[string]*BCFunction{"f": {}}}, []string{}},
		{"inline bodies", &BCProgram{Main: []BCInstruction{{Op: OpTryLet}, {Op: OpEnv}, {Op: OpSpawnAgent}, {Op: OpReadFile}, {Op: OpHttpRoute}, {Op: OpExec}}}, []string{"environment", "filesystem", "network", "process"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := RequiredCapabilities(tt.p)
			if !reflect.DeepEqual(r.Capabilities, tt.want) {
				t.Fatalf("got %v want %v", r.Capabilities, tt.want)
			}
			a, _ := json.Marshal(r)
			b, _ := json.Marshal(RequiredCapabilities(tt.p))
			if string(a) != string(b) {
				t.Fatal("nondeterministic")
			}
			if tt.name == "empty" && string(a) != `{"capabilities":[],"sites":[]}` {
				t.Fatal(string(a))
			}
		})
	}
	p := &BCProgram{Main: []BCInstruction{{Op: OpEnv}}, Functions: map[string]*BCFunction{"z": {Instructions: []BCInstruction{{Op: OpExec}}}, "a": {Instructions: []BCInstruction{{Op: OpFetch}}}}}
	r := RequiredCapabilities(p)
	for i, want := range []string{"main", "a", "z"} {
		if r.Sites[i].Function != want {
			t.Fatal(r.Sites)
		}
	}
	for op, spec := range Registry {
		if spec.Capability != "" {
			r := RequiredCapabilities(&BCProgram{Main: []BCInstruction{{Op: op}}})
			found := false
			for _, c := range r.Capabilities {
				found = found || c == string(spec.Capability)
			}
			if !found {
				t.Fatalf("missing %s", spec.Name)
			}
		}
	}
}

func TestRequiredCapabilitiesCompiledInlineBodies(t *testing.T) {
	sources := []string{
		`(cli_app (try_let (x (env "PATH")) (catch err (print err)) (print x)))`,
		`(cli_app (spawn_agent "a" (task "task" (print (env "PATH")))))`,
		`(http_server "8080" (route "/" (lambda (req) (print (env "PATH")))))`,
	}
	for _, source := range sources {
		prog := CompileToBytecode(parser.NewParser(lexer.NewLexer(source), "inline.howl").ParseExpression())
		report := RequiredCapabilities(prog)
		found := false
		for _, site := range report.Sites {
			if site.Opcode == "ENV" && site.Function == "main" {
				found = true
			}
		}
		if !found {
			t.Fatalf("inline body omitted: %s; %+v", source, report)
		}
	}
}
