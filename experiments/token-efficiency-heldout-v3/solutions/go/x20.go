//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func read(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func writeLines(lines []string) {
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	var out []string
	words := 0
	lines := append(strings.Split(read("in/doc.txt"), "\n"), "")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if words > 0 {
				out = append(out, fmt.Sprintf("%d: %d", len(out)+1, words))
			}
			words = 0
			continue
		}
		words += len(strings.Fields(line))
	}
	writeLines(out)
}
