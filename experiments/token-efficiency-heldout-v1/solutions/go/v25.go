//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/sentence.txt")
	if err != nil {
		panic(err)
	}
	longest := ""
	for _, word := range strings.Split(strings.TrimSpace(string(data)), " ") {
		if len(word) > len(longest) {
			longest = word
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(longest+"\n"), 0o644); err != nil {
		panic(err)
	}
}
