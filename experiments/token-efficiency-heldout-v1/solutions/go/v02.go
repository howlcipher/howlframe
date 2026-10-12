//go:build ignore

package main

import (
	"os"
	"slices"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/poem.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		words := strings.Split(line, " ")
		slices.Reverse(words)
		lines[i] = strings.Join(words, " ")
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
