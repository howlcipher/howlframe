//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/tags.txt")
	if err != nil {
		panic(err)
	}
	seen := map[string]bool{}
	unique := []string{}
	for _, tag := range strings.Split(string(data), ";") {
		if !seen[tag] {
			seen[tag] = true
			unique = append(unique, tag)
		}
	}
	out, err := json.MarshalIndent(unique, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
