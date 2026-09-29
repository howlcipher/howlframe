package httpreq

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryHeaderAndPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tasks/9?status=open&status=closed&empty=", nil)
	req.Header.Add("Authorization", "Bearer z")
	req.Header.Add("Authorization", "Bearer later")
	req = WithParams(req, map[string]string{"id": "9"})

	got, err := Query(req, "status")
	if err != nil || got != "open" {
		t.Fatalf("query status = %q, %v", got, err)
	}
	got, err = Query(req, "missing")
	if err != nil || got != "" {
		t.Fatalf("query miss = %q, %v", got, err)
	}
	got, err = Query(req, "empty")
	if err != nil || got != "" {
		t.Fatalf("query empty = %q, %v", got, err)
	}
	got, err = Query(req, "Status")
	if err != nil || got != "" {
		t.Fatalf("query names are case-sensitive, got %q, %v", got, err)
	}

	got, err = Header(req, "authorization")
	if err != nil || got != "Bearer z" {
		t.Fatalf("header = %q, %v", got, err)
	}
	got, err = Header(req, "X-Missing")
	if err != nil || got != "" {
		t.Fatalf("header miss = %q, %v", got, err)
	}

	got, err = Path(req, "id")
	if err != nil || got != "9" {
		t.Fatalf("path = %q, %v", got, err)
	}
	got, err = Path(req, "nope")
	if err != nil || got != "" {
		t.Fatalf("path miss = %q, %v", got, err)
	}
	plain := httptest.NewRequest(http.MethodGet, "/health", nil)
	got, err = Path(plain, "id")
	if err != nil || got != "" {
		t.Fatalf("path without captures = %q, %v", got, err)
	}
}

func TestReadsFailClosed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	cases := []struct {
		name string
		fn   func() (string, error)
	}{
		{"query receiver", func() (string, error) { return Query("nope", "status") }},
		{"header receiver", func() (string, error) { return Header(nil, "Authorization") }},
		{"path receiver", func() (string, error) { return Path(1, "id") }},
		{"query name", func() (string, error) { return Query(req, 1) }},
		{"header empty name", func() (string, error) { return Header(req, "") }},
		{"path empty name", func() (string, error) { return Path(req, "") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.fn()
			he, ok := err.(*Error)
			if !ok || he.Code != "TYPE_ERROR" {
				t.Fatalf("err = %v, want TYPE_ERROR", err)
			}
		})
	}
}

func TestParseAndMatchRoute(t *testing.T) {
	pattern, err := ParseRoute("/tasks/{id}/notes/{note}")
	if err != nil || !pattern.Parametric {
		t.Fatalf("parse = %#v, %v", pattern, err)
	}
	params, ok := pattern.Match("/tasks/foo/../9/notes/abc")
	if !ok || params["id"] != "9" || params["note"] != "abc" {
		t.Fatalf("match = %#v, %v", params, ok)
	}
	if _, ok := pattern.Match("/tasks/9/notes/abc/"); ok {
		t.Fatal("trailing slash matched a pattern that has none")
	}
	if _, ok := pattern.Match("/tasks/9"); ok {
		t.Fatal("short path matched")
	}

	literal, err := ParseRoute("/health")
	if err != nil || literal.Parametric {
		t.Fatalf("literal = %#v, %v", literal, err)
	}

	bad := []string{"", "/tasks/{", "/tasks/id}", "/tasks/{}", "/tasks/{1id}", "/tasks/{id}/{id}", "/tasks/pre{id}"}
	for _, path := range bad {
		if _, err := ParseRoute(path); err == nil {
			t.Fatalf("ParseRoute(%q) succeeded", path)
		} else if he, ok := err.(*Error); !ok || he.Code != "TYPE_ERROR" {
			t.Fatalf("ParseRoute(%q) err = %v, want TYPE_ERROR", path, err)
		}
	}
}

func TestServerLiteralWinsAndPatternsCapture(t *testing.T) {
	srv := NewServer(http.NewServeMux())
	var got string
	if err := srv.Handle("/tasks/new", func(w http.ResponseWriter, r *http.Request) {
		got = "literal"
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("literal"))
	}); err != nil {
		t.Fatal(err)
	}
	if err := srv.Handle("/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := Path(r, "id")
		if err != nil {
			t.Errorf("path read: %v", err)
		}
		got = id
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(id))
	}); err != nil {
		t.Fatal(err)
	}
	if err := srv.Handle("/tasks/{task_id}", nil); err == nil {
		t.Fatal("nil handler was accepted")
	}
	if err := srv.Handle("/tasks/{other}", func(http.ResponseWriter, *http.Request) {}); err == nil {
		t.Fatal("duplicate skeleton was accepted")
	} else if he, ok := err.(*Error); !ok || he.Code != "RUNTIME_ERROR" {
		t.Fatalf("duplicate err = %v", err)
	}

	literal := httptest.NewRecorder()
	srv.ServeHTTP(literal, httptest.NewRequest(http.MethodGet, "/tasks/new", nil))
	if literal.Body.String() != "literal" || got != "literal" {
		t.Fatalf("literal route = %q, got %q", literal.Body.String(), got)
	}

	captured := httptest.NewRecorder()
	srv.ServeHTTP(captured, httptest.NewRequest(http.MethodGet, "/tasks/foo/../9", nil))
	if captured.Body.String() != "9" {
		t.Fatalf("pattern route = %q", captured.Body.String())
	}

	miss := httptest.NewRecorder()
	srv.ServeHTTP(miss, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if miss.Code != http.StatusNotFound {
		t.Fatalf("miss status = %d", miss.Code)
	}
}
