//go:build ignore

package main

import (
	"os"
	"sort"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/poem.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	sort.Strings(lines)
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
