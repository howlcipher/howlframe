package vm

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/howlcipher/howlframe/internal/backend/gogen"
	"github.com/howlcipher/howlframe/internal/backend/javascript"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

// map_keys order is UTF-8 byte order on the interpreter, the bytecode VM,
// the Go backend, and JavaScript. The BMP fixture is a case where UTF-16
// code-unit order already agrees. The non-BMP fixture does not: U+F000
// sorts before U+1F000 and U+1F600 in byte order, and after them in UTF-16.
func TestMapKeysSortParity(t *testing.T) {
	cases := []mapKeySortCase{
		{
			name: "bmp",
			file: "map_keys_sort_bmp.howl",
			keys: []string{"gamma", "beta", "alpha", "é"},
		},
		{
			name:     "nonbmp",
			file:     "map_keys_sort_nonbmp.howl",
			keys:     []string{"a", "é", "\uF000", "\U0001F000", "\U0001F600"},
			diverges: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			byteOrder := sortedJoin(tc.keys, utf8Less)
			utf16Order := sortedJoin(tc.keys, utf16Less)
			if tc.diverges {
				if byteOrder == utf16Order {
					t.Fatalf("fixture keys do not distinguish UTF-8 byte order from UTF-16: %q", byteOrder)
				}
			} else if byteOrder != utf16Order {
				t.Fatalf("BMP fixture diverges: byte %q utf16 %q", byteOrder, utf16Order)
			}
			want := byteOrder + "\n"
			source := string(mustRead(t, filepath.Join("..", "..", "tests", "fixtures", tc.file)))

			t.Run("interpreter", func(t *testing.T) {
				node, _ := parseAndCompile(t, source)
				var out, errOut bytes.Buffer
				if exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
					t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
				}
				if out.String() != want {
					t.Fatalf("stdout = %q, want byte order %q (UTF-16 would be %q)", out.String(), want, utf16Order+"\n")
				}
			})

			t.Run("bytecode", func(t *testing.T) {
				_, prog := parseAndCompile(t, source)
				var out, errOut bytes.Buffer
				if exitCode := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
					t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
				}
				if out.String() != want {
					t.Fatalf("stdout = %q, want byte order %q (UTF-16 would be %q)", out.String(), want, utf16Order+"\n")
				}
			})

			t.Run("go", func(t *testing.T) {
				code := generateCheckedGoOrder(t, source)
				if !strings.Contains(code, "sort.Strings(_keys)") {
					t.Fatalf("Go map_keys left sort.Strings:\n%s", code)
				}
				if got := goRunOrder(t, code); got != want {
					t.Fatalf("stdout = %q, want byte order %q (UTF-16 would be %q)\nsource:\n%s", got, want, utf16Order+"\n", code)
				}
			})

			t.Run("js", func(t *testing.T) {
				code := generateCheckedJSOrder(t, asWebApp(t, source))
				if !strings.Contains(code, "sort(howlFrameCompareUTF8)") {
					t.Fatalf("JavaScript map_keys left UTF-8 comparison:\n%s", code)
				}
				if strings.Contains(code, ".sort()") {
					t.Fatalf("JavaScript map_keys still uses the default UTF-16 sort:\n%s", code)
				}
				stdout, stderr, err := runNodeOrder(t, code)
				if err != nil {
					t.Fatalf("node: %v\nstderr=%s\nsource:\n%s", err, stderr, code)
				}
				if stdout != want {
					t.Fatalf("stdout = %q, want byte order %q (UTF-16 would be %q)\nsource:\n%s", stdout, want, utf16Order+"\n", code)
				}
			})
		})
	}
}

type mapKeySortCase struct {
	name     string
	file     string
	keys     []string
	diverges bool
}

func sortedJoin(keys []string, less func(a, b string) bool) string {
	cp := append([]string(nil), keys...)
	sort.Slice(cp, func(i, j int) bool { return less(cp[i], cp[j]) })
	return strings.Join(cp, ",")
}

func utf8Less(a, b string) bool {
	return a < b
}

func utf16Less(a, b string) bool {
	ua := utf16.Encode([]rune(a))
	ub := utf16.Encode([]rune(b))
	n := len(ua)
	if len(ub) < n {
		n = len(ub)
	}
	for i := 0; i < n; i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

func asWebApp(t *testing.T, source string) string {
	t.Helper()
	trimmed := strings.TrimSpace(source)
	const root = "(cli_app"
	if !strings.HasPrefix(trimmed, root) {
		t.Fatalf("fixture root = %q, want cli_app", trimmed)
	}
	return "(web_app" + trimmed[len(root):] + "\n"
}

func generateCheckedGoOrder(t *testing.T, source string) string {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "map_keys_sort.howl").ParseExpression()
	checker.Check(root)
	code, _ := gogen.GenerateCode(root)
	return code
}

func generateCheckedJSOrder(t *testing.T, source string) string {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "map_keys_sort.howl").ParseExpression()
	checker.Check(root)
	code, _ := javascript.GenerateJSCode(root)
	return code
}

func goRunOrder(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", path)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, out)
	}
	return string(out)
}

func runNodeOrder(t *testing.T, source string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "app.js")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
