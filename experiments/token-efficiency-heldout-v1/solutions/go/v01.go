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
	first := make([]string, len(lines))
	for i, line := range lines {
		first[i] = strings.Split(line, " ")[0]
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(first, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
