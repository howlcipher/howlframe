package vm

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
)

type customErrReader struct {
	msg string
}

func (c *customErrReader) Read(p []byte) (n int, err error) {
	return 0, &testingCustomError{msg: c.msg}
}

type testingCustomError struct {
	msg string
}

func (e *testingCustomError) Error() string {
	return e.msg
}

type repeatByteReader byte

func (r repeatByteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

// TestBytecodeParseJSONRequestBody documents the supported standalone HTTP
// composition: parse_json reads req.body inside a route context, try_let
// handles malformed JSON, and decoded object/list values remain usable by
// map_get and res_json. It intentionally exercises the VM's route-request
// binding rather than inventing a separate req_body construct.
func TestBytecodeParseJSONRequestBody(t *testing.T) {
	const program = `(cli_app
  (try_let (body (parse_json Any req.body))
    (catch err (res_json 400 (dict ("error" "invalid_json"))) )
    (res_json 200 (dict ("title" (map_get body "title")) ("tags" (map_get body "tags"))))))`

	_, bytecodeProgram := parseAndCompile(t, program)

	tests := []struct {
		name       string
		body       io.Reader
		wantStatus int
		wantJSON   map[string]any
	}{
		{
			name:       "object and list values",
			body:       strings.NewReader(`{"title":"Ship v0.1.1","tags":["release","consumer"]}`),
			wantStatus: 200,
			wantJSON: map[string]any{
				"title": "Ship v0.1.1",
				"tags":  []any{"release", "consumer"},
			},
		},
		{
			name:       "malformed JSON follows catch branch",
			body:       strings.NewReader(`{`),
			wantStatus: 400,
			wantJSON:   map[string]any{"error": "invalid_json"},
		},
		{
			name:       "empty body follows catch branch",
			body:       strings.NewReader(""),
			wantStatus: 400,
			wantJSON:   map[string]any{"error": "invalid_json"},
		},
		{
			name:       "simulated read error follows catch branch",
			body:       &customErrReader{msg: "connection reset by peer"},
			wantStatus: 400,
			wantJSON:   map[string]any{"error": "invalid_json"},
		},
		{
			name:       "oversized body exceeds 10MB limit follows catch branch",
			body:       io.LimitReader(repeatByteReader('{'), 10*1024*1024+100),
			wantStatus: 400,
			wantJSON:   map[string]any{"error": "invalid_json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/tasks", tt.body)
			recorder := httptest.NewRecorder()
			env := NewBcEnv(nil)
			env.vars["req"] = req
			env.vars["w"] = recorder

			machine := &BCVM{
				prog:        bytecodeProgram,
				env:         env,
				insts:       bytecodeProgram.Main,
				stores:      newBCStoreRegistry(),
				Limits:      DefaultLimits,
				AllowedCaps: []capability.Capability{capability.Network},
			}
			machine.run(machine.insts, env)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			var got map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatalf("response is not JSON: %v; body=%s", err, recorder.Body.String())
			}
			if !jsonEqual(got, tt.wantJSON) {
				t.Fatalf("response = %#v, want %#v", got, tt.wantJSON)
			}
		})
	}
}

func jsonEqual(got, want map[string]any) bool {
	gotJSON, err := json.Marshal(got)
	if err != nil {
		return false
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		return false
	}
	return string(gotJSON) == string(wantJSON)
}
