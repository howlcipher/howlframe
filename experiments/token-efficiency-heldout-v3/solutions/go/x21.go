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
	var rows [][]string
	for _, line := range strings.Split(read("in/grid.csv"), "\n") {
		rows = append(rows, strings.Split(line, ","))
	}
	var out []string
	for j := range rows[0] {
		col := make([]string, len(rows))
		for i, r := range rows {
			col[i] = r[j]
		}
		out = append(out, strings.Join(col, ","))
	}
	writeLines(out)
}
