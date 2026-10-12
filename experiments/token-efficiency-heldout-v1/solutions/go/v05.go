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
	var kept []string
	for _, line := range lines {
		if len(strings.Split(line, " ")) == 3 {
			kept = append(kept, line)
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
