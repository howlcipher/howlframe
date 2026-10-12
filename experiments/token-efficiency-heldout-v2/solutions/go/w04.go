//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/headings.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		words := strings.Split(line, " ")
		for j, w := range words {
			if w != "" {
				words[j] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
		lines[i] = strings.Join(words, " ")
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
