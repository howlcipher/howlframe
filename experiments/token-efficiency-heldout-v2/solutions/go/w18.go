//go:build ignore

package main

import (
	"os"
	"strings"
	"unicode"
)

func main() {
	data, err := os.ReadFile("in/idents.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		var b strings.Builder
		for _, c := range line {
			if unicode.IsUpper(c) {
				b.WriteRune('_')
				c = unicode.ToLower(c)
			}
			b.WriteRune(c)
		}
		lines[i] = b.String()
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
