//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
)

func readJSON(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		panic(err)
	}
}

func writeLines(lines []string) {
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	var tickets []struct {
		ID   string
		Meta struct{ Tags []string }
	}
	readJSON("in/tickets.json", &tickets)
	var out []string
	for _, t := range tickets {
		if slices.Contains(t.Meta.Tags, "urgent") {
			out = append(out, t.ID)
		}
	}
	writeLines(out)
}
