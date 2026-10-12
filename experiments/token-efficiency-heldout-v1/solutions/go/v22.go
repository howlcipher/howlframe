//go:build ignore

package main

import (
	"os"
	"slices"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/word.txt")
	if err != nil {
		panic(err)
	}
	word := strings.TrimSpace(string(data))
	chars := []rune(word)
	slices.Reverse(chars)
	answer := "no"
	if string(chars) == word {
		answer = "yes"
	}
	if err := os.WriteFile("out/result.txt", []byte(answer+"\n"), 0o644); err != nil {
		panic(err)
	}
}
