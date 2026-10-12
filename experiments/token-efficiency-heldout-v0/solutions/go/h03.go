//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/text.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		lines[i] = fmt.Sprintf("%d: %s", i+1, line)
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
