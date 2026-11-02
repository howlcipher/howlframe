package gogen

import (
	"go/token"
	"reflect"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func allocateGoTestFunctionNames(descriptions []string) []string {
	reserved := make(map[string]struct{}, len(descriptions))
	names := make([]string, len(descriptions))
	for i, description := range descriptions {
		names[i] = allocateGoTestFunctionName(description, reserved)
	}
	return names
}

func TestGenerateCodeTestNamesAreDeterministic(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(cli_app
	  (test "twin" (assert true))
	  (test "twin" (assert true))
	  (test "twin 2" (assert true)))`), "names.howl").ParseExpression()
	checker.Check(root)

	_, first := GenerateCode(root)
	_, second := GenerateCode(root)
	if first != second {
		t.Fatal("identical input produced different generated Go")
	}

	previous := -1
	for _, name := range []string{"Test_twin", "Test_twin_2", "Test_twin_2_2"} {
		if count := strings.Count(first, "func "+name+"("); count != 1 {
			t.Errorf("generated function %s appears %d times, want once", name, count)
		}
		position := strings.Index(first, "func "+name+"(")
		if position <= previous {
			t.Errorf("function %s is not in source order", name)
		}
		previous = position
	}
}

func TestAllocateGoTestFunctionNames(t *testing.T) {
	tests := []struct {
		name string
		desc []string
		want []string
	}{
		{
			name: "original collision",
			desc: []string{"twin", "twin", "twin 2"},
			want: []string{"Test_twin", "Test_twin_2", "Test_twin_2_2"},
		},
		{
			name: "reversed order",
			desc: []string{"twin 2", "twin", "twin"},
			want: []string{"Test_twin_2", "Test_twin", "Test_twin_3"},
		},
		{
			name: "punctuation",
			desc: []string{"a b", "a-b", "a_b"},
			want: []string{"Test_a_b", "Test_a_b_2", "Test_a_b_3"},
		},
		{
			name: "nested suffix collision",
			desc: []string{"x", "x", "x 2", "x 2"},
			want: []string{"Test_x", "Test_x_2", "Test_x_2_2", "Test_x_2_3"},
		},
		{
			name: "repeated base",
			desc: []string{"same", "same", "same", "same", "same", "same"},
			want: []string{"Test_same", "Test_same_2", "Test_same_3", "Test_same_4", "Test_same_5", "Test_same_6"},
		},
		{
			name: "empty descriptions",
			desc: []string{"", ""},
			want: []string{"Test", "Test_2"},
		},
		{
			name: "punctuation only",
			desc: []string{"!", "?", "!!!"},
			want: []string{"Test", "Test_2", "Test_3"},
		},
		{
			name: "numeric",
			desc: []string{"2", "2", "2 2"},
			want: []string{"Test_2", "Test_2_2", "Test_2_2_2"},
		},
		{
			name: "unicode",
			desc: []string{"猫", "猫"},
			want: []string{"Test_猫", "Test_猫_2"},
		},
		{
			name: "interleaved",
			desc: []string{"one", "same", "other", "same", "one 2"},
			want: []string{"Test_one", "Test_same", "Test_other", "Test_same_2", "Test_one_2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allocateGoTestFunctionNames(tt.desc)
			if len(got) != len(tt.desc) {
				t.Fatalf("allocated %d names for %d test blocks", len(got), len(tt.desc))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("names = %#v, want %#v", got, tt.want)
			}

			seen := make(map[string]struct{}, len(got))
			for _, name := range got {
				if !token.IsIdentifier(name) {
					t.Errorf("%q is not a valid Go identifier", name)
				}
				if _, exists := seen[name]; exists {
					t.Errorf("duplicate generated function name %q", name)
				}
				seen[name] = struct{}{}
			}

			if again := allocateGoTestFunctionNames(tt.desc); !reflect.DeepEqual(again, got) {
				t.Fatalf("allocation is nondeterministic: first %#v, second %#v", got, again)
			}
		})
	}
}
