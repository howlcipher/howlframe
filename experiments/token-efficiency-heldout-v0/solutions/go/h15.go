//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/text.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	longest := lines[0]
	for _, line := range lines[1:] {
		if len(line) > len(longest) {
			longest = line
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(longest+"\n"), 0o644); err != nil {
		panic(err)
	}
}
