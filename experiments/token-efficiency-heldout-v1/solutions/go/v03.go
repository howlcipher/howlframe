//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/poem.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if len(line) > 3 {
			lines[i] = line[:3]
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
