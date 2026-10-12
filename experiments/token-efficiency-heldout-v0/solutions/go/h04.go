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
	var kept []string
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
