//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/title.txt")
	if err != nil {
		panic(err)
	}
	words := strings.Fields(strings.ToLower(strings.TrimSpace(string(data))))
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(words, "-")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
