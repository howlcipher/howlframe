package gogen

import (
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"strings"
	"testing"
)

func TestGenerateCodePreservesFloatComparison(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(cli_app (let (score 1.0) (if (> score 0.8) (print "yes") (print "no"))))`), "float.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	if !strings.Contains(code, "score > 0.8") {
		t.Fatalf("generated code did not preserve float comparison:\n%s", code)
	}
}

func TestGenerateCodeRequestSurface(t *testing.T) {
	pattern := parser.NewParser(lexer.NewLexer(`(http_server 8080
  (route "/health" (lambda (req) (res 200 "text/plain" "ok")))
  (route "/tasks/{id}" (lambda (req)
    (let (id (req_path req "id"))
      (let (status (req_query req "status"))
        (let (auth (req_header req "Authorization"))
          (res 200 "text/plain" id)))))))`), "request.howl").ParseExpression()
	checker.Check(pattern)
	code, _ := GenerateCode(pattern)
	for _, want := range []string{
		"httpreq.NewServer",
		"howlHTTP.Handle",
		"httpreq.Path",
		"httpreq.Query",
		"httpreq.Header",
		`ListenAndServe(":8080", howlHTTP)`,
		"panic(_err.Error())",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated Go is missing %q:\n%s", want, code)
		}
	}

	literal := parser.NewParser(lexer.NewLexer(
		`(http_server 8080 (route "/health" (lambda (req) (res 200 "text/plain" "ok"))))`),
		"literal.howl").ParseExpression()
	checker.Check(literal)
	literalCode, _ := GenerateCode(literal)
	if strings.Contains(literalCode, "httpreq") || strings.Contains(literalCode, "howlHTTP") {
		t.Fatalf("literal server changed dispatch:\n%s", literalCode)
	}
	if !strings.Contains(literalCode, `http.HandleFunc("/health"`) || !strings.Contains(literalCode, `ListenAndServe(":8080", nil)`) {
		t.Fatalf("literal server lost HandleFunc:\n%s", literalCode)
	}
}

func TestGenerateCodeMapKeysSortsBothDictShapes(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(
		`(cli_app (let (counts (dict ("beta" "2") ("alpha" "1"))) (print (str_join (map_keys counts) ","))))`),
		"keys.howl").ParseExpression()
	checker.Check(root)
	code, _ := GenerateCode(root)
	for _, want := range []string{
		"case map[string]string:",
		"case map[string]any:",
		"sort.Strings(_keys)",
		"TYPE_ERROR: map_keys expected dict",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated Go is missing %q:\n%s", want, code)
		}
	}
}
