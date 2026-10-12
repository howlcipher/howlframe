//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/brackets.txt")
	if err != nil {
		panic(err)
	}
	pairs := map[rune]rune{')': '(', ']': '[', '}': '{'}
	var stack []rune
	result := ""
	for i, ch := range strings.TrimSpace(string(data)) {
		if strings.ContainsRune("([{", ch) {
			stack = append(stack, ch)
		} else if open, ok := pairs[ch]; ok {
			if len(stack) == 0 || stack[len(stack)-1] != open {
				result = fmt.Sprintf("error at %d", i)
				break
			}
			stack = stack[:len(stack)-1]
		}
	}
	if result == "" {
		result = "ok"
		if len(stack) > 0 {
			result = "unclosed"
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(result+"\n"), 0o644); err != nil {
		panic(err)
	}
}
