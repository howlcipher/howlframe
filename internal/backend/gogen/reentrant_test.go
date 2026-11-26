package gogen

import (
	"sync"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestGenerateGoCodeConcurrentReentrancy(t *testing.T) {
	programs := []string{
		`(cli_app (do (print (env "PORT")) (print (read_file "a.txt")) (write_file "b.txt" "x") (mkdir "d") (print (fetch "https://example.com" "GET"))))`,
		`(http_server 8080 (route "/items/{id}" (lambda (req) (let (id (req_path req "id")) (res 200 "text/html" (html_escape id))))))`,
		`(cli_app (let (m (dict ("a" "1"))) (do (print (map_get m "a")) (print (is_nil (map_get m "b"))))))`,
		`(cli_app (let (xs (list "1" "2")) (do (print (list_get xs (to_int "0"))) (print (list_len xs)) (print (/ 10 2)))))`,
	}
	expected := make([]string, len(programs))
	for i, src := range programs {
		root := parser.NewParser(lexer.NewLexer(src), "test.howl").ParseExpression()
		checker.Check(root)
		code, _ := GenerateCode(root)
		expected[i] = code
	}

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for iter := 0; iter < 15; iter++ {
				idx := (id + iter) % len(programs)
				root := parser.NewParser(lexer.NewLexer(programs[idx]), "test.howl").ParseExpression()
				checker.Check(root)
				code, _ := GenerateCode(root)
				if code != expected[idx] {
					t.Errorf("concurrent codegen mismatch for program %d", idx)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}
