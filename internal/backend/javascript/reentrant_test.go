package javascript

import (
	"sync"
	"testing"

	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestGenerateJSCodeConcurrentReentrancy(t *testing.T) {
	programs := []string{
		`(web_app (print (env "PORT")) (fetch "https://example.com" "GET"))`,
		`(web_app (dom_query "#root") (print (= (list 1 2) (list 1 2))))`,
		`(web_app (print (+ 1 2)))`,
		`(web_app (on_scroll (lambda (x) (print (encode_json (dict ("a" x)))))))`,
	}
	expected := make([]string, len(programs))
	for i, src := range programs {
		root := parser.NewParser(lexer.NewLexer(src), "test.howl").ParseExpression()
		code, _ := GenerateJSCode(root)
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
				code, _ := GenerateJSCode(root)
				if code != expected[idx] {
					t.Errorf("concurrent codegen mismatch for program %d", idx)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}
