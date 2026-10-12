//go:build ignore

package main

import (
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
	if err := os.WriteFile("out/result.csv", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	lines := strings.Split(read("in/sites.csv"), "\n")
	out := []string{lines[0]}
	var prev []string
	for _, line := range lines[1:] {
		cells := strings.Split(line, ",")
		for j, c := range cells {
			if c == "" {
				cells[j] = prev[j]
			}
		}
		prev = cells
		out = append(out, strings.Join(cells, ","))
	}
	writeLines(out)
}
