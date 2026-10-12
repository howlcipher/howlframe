//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/team.csv")
	if err != nil {
		panic(err)
	}
	var rows [][]string
	width := 0
	for _, line := range strings.Split(string(data), "\n")[1:] {
		r := strings.Split(line, ",")
		rows = append(rows, r)
		width = max(width, len(r[0]))
	}
	var out []string
	for _, r := range rows {
		out = append(out, fmt.Sprintf("%-*s | %s", width, r[0], r[1]))
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
